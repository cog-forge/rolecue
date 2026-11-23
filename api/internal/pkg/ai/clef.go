package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Clef is Cloudflare's decision model on Workers AI (KAN-274). It answers typed questions about a state with
// probabilities and writes no text, so it is not a chat model: it sits in front of a chat route as a
// decision, never inside one. Request and response shapes follow
// developers.cloudflare.com/workers-ai/models/clef (schema-input.json, schema-output.json); the REST
// envelope {result, success, errors, messages} was confirmed by the first live call on 2026-11-18.

const (
	cloudflareAPIBase = "https://api.cloudflare.com"
	// clefQuestionID is the only question this adapter asks.
	clefQuestionID = "ask_follow_up"
	// maxClefReplyBytes bounds the decoded reply; a valid one is a few hundred bytes.
	maxClefReplyBytes = 64 << 10
)

// ClefYesNo is one noul (yes/no) question. Instructions, Yes and No are fixed text; untrusted input goes
// only into the state.
type ClefYesNo struct {
	Instructions string
	Yes          string
	No           string
}

// ClefUsage is the token usage Clef reports; only input tokens are billed.
type ClefUsage struct {
	InputTokens  int
	OutputTokens int
}

type clefClient struct {
	endpoint, token, model string
	client                 *http.Client
}

func newClefClient(base, accountID, token, model string, timeout time.Duration) *clefClient {
	return &clefClient{
		endpoint: base + "/client/v4/accounts/" + accountID + "/ai/run/@cf/cloudflare/" + model,
		token:    token,
		model:    model,
		// One request per call, no redirects; the caller owns retries and fallback.
		client: &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

type clefNoul struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

// YesProbability asks one yes/no question about state and returns the probability of yes. Errors carry a
// status or a classified reason, never the response body, which may echo the state.
func (c *clefClient) YesProbability(ctx context.Context, state any, q ClefYesNo) (float64, ClefUsage, error) {
	answers, usage, err := c.yesProbabilities(ctx, state, map[string]ClefYesNo{clefQuestionID: q})
	if err != nil {
		return 0, usage, err
	}
	return answers[clefQuestionID], usage, nil
}

// maxClefQuestions is the documented limit of questions in one request.
const maxClefQuestions = 64

// yesProbabilities asks up to maxClefQuestions yes/no questions about one state in one request. Every id must
// come back as a noul answer with a probability in [0, 1], or the whole reply is invalid output.
func (c *clefClient) yesProbabilities(ctx context.Context, state any, questions map[string]ClefYesNo) (map[string]float64, ClefUsage, error) {
	if len(questions) == 0 || len(questions) > maxClefQuestions {
		return nil, ClefUsage{}, errors.New("clef request needs 1 to 64 questions")
	}
	nouls := make(map[string]clefNoul, len(questions))
	for id, q := range questions {
		var criteria map[string]string
		if q.Yes != "" || q.No != "" {
			criteria = map[string]string{"true": q.Yes, "false": q.No}
		}
		nouls[id] = clefNoul{Type: "noul", Instructions: q.Instructions, Criteria: criteria}
	}
	body, err := json.Marshal(struct {
		Model     string              `json:"model"`
		State     any                 `json:"state"`
		Questions map[string]clefNoul `json:"questions"`
	}{c.model, state, nouls})
	if err != nil {
		return nil, ClefUsage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, ClefUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "rolecue/1.0")
	resp, err := c.client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ClefUsage{}, ctxErr
		}
		return nil, ClefUsage{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		delay, _ := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64)
		return nil, ClefUsage{}, &HTTPError{Status: resp.StatusCode, RetryAfter: time.Duration(delay * float64(time.Second))}
	}
	var out struct {
		Success bool `json:"success"`
		Result  *struct {
			Answers map[string]struct {
				Type string   `json:"type"`
				Noul *float64 `json:"noul"`
			} `json:"answers"`
			Usage struct {
				Input  int `json:"input_tokens"`
				Output int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxClefReplyBytes)).Decode(&out); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ClefUsage{}, ctxErr
		}
		return nil, ClefUsage{}, ErrInvalidOutput
	}
	if !out.Success || out.Result == nil {
		return nil, ClefUsage{}, ErrInvalidOutput
	}
	usage := ClefUsage{InputTokens: out.Result.Usage.Input, OutputTokens: out.Result.Usage.Output}
	probabilities := make(map[string]float64, len(questions))
	for id := range questions {
		answer, ok := out.Result.Answers[id]
		if !ok || answer.Type != "noul" || answer.Noul == nil {
			return nil, usage, ErrInvalidOutput
		}
		p := *answer.Noul
		if math.IsNaN(p) || p < 0 || p > 1 {
			return nil, usage, ErrInvalidOutput
		}
		probabilities[id] = p
	}
	return probabilities, usage, nil
}

var errClefNotConfigured = errors.New("clef decision route is not configured")
