package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// AzureName is stored in voices.provider for voices synced from Azure AI Speech.
const AzureName = "azure"

const (
	defaultTimeout = 30 * time.Second
	// maxResponseBytes bounds the voice list (about 1 MiB in practice) so a bad answer can't fill memory.
	maxResponseBytes = 8 << 20
)

// azureRegion is a lowercase Azure region name such as "eastus"; it becomes part of the host name.
var azureRegion = regexp.MustCompile(`^[a-z0-9]{2,40}$`)

// Azure lists the voices of one Azure AI Speech region.
// https://learn.microsoft.com/azure/ai-services/speech-service/rest-text-to-speech#get-a-list-of-voices
type Azure struct {
	key     string
	baseURL string
	client  *http.Client
}

var _ service.VoiceProvider = (*Azure)(nil)

// NewAzure builds the Azure adapter. Without a key or region it still builds, and ListVoices reports
// service.ErrProviderNotConfigured, so the API starts in environments without Azure credentials.
func NewAzure(key, region string, timeout time.Duration) (*Azure, error) {
	a := &Azure{key: key}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	a.client = &http.Client{Timeout: timeout}
	if key == "" || region == "" {
		return a, nil
	}
	if !azureRegion.MatchString(region) {
		return nil, fmt.Errorf("invalid Azure speech region %q", region)
	}
	a.baseURL = "https://" + region + ".tts.speech.microsoft.com"
	return a, nil
}

// withBaseURL points the adapter at another host; tests use it to reach an httptest server.
func (a *Azure) withBaseURL(url string) *Azure {
	a.baseURL = url
	return a
}

func (a *Azure) Name() string { return AzureName }

// azureVoice holds the fields of the Azure answer the catalog uses.
type azureVoice struct {
	ShortName   string `json:"ShortName"`
	DisplayName string `json:"DisplayName"`
	LocalName   string `json:"LocalName"`
	Gender      string `json:"Gender"`
	Locale      string `json:"Locale"`
	VoiceType   string `json:"VoiceType"`
}

// standardVoiceType is the VoiceType of the neural voices Azure lists (the REST docs show "Neural").
const standardVoiceType = "Neural"

// isStandardNeural reports whether v belongs in the catalog: a standard neural voice. The catalog feeds the
// interviewer avatar, which needs <mstts:viseme>, and Microsoft documents that the HD voices do not support
// it. Dropped:
//   - HD voices, named "persona:DragonHD...Neural" (for example en-US-Ava:DragonHDLatestNeural) or
//     "locale-voice:DragonHDOmniLatestNeural";
//   - multilingual voices ("...MultilingualNeural", including the Turbo ones): the language-support page lists
//     them as their own "Multilingual" type, apart from "Standard", and does not state viseme support for them;
//   - anything whose VoiceType is not "Neural".
//
// https://learn.microsoft.com/azure/ai-services/speech-service/high-definition-voices
// https://learn.microsoft.com/azure/ai-services/speech-service/language-support?tabs=tts
func isStandardNeural(v azureVoice) bool {
	if !strings.EqualFold(v.VoiceType, standardVoiceType) {
		return false
	}
	name := strings.ToLower(v.ShortName)
	return !strings.Contains(name, ":") &&
		!strings.Contains(name, "dragonhd") &&
		!strings.HasSuffix(name, "multilingualneural")
}

// ListVoices returns the standard neural voices of the region (see isStandardNeural). Filtering happens here, so
// the service validates and writes only voices the catalog may offer; an HD or multilingual voice that an earlier
// sync stored is simply no longer offered and the sync deactivates it.
func (a *Azure) ListVoices(ctx context.Context) (voices []domain.ProviderVoice, err error) {
	if a.key == "" || a.baseURL == "" {
		return nil, service.ErrProviderNotConfigured
	}
	done := logger.Start(ctx, "voicecatalog.list_azure", logger.String("provider", AzureName))
	var failure []zap.Field
	defer func() {
		if err != nil {
			done(err, failure...)
			return
		}
		done(nil, logger.Int("voices", len(voices)))
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/cognitiveservices/voices/list", nil)
	if err != nil {
		return nil, fmt.Errorf("build azure voices request: %w", err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", a.key)
	req.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		// The error carries the URL but never the key, which travels in a header.
		failure = []zap.Field{logger.String("reason", transportReason(err))}
		return nil, fmt.Errorf("azure voices request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		failure = []zap.Field{logger.String("reason", statusReason(resp.StatusCode)), logger.Int("status_code", resp.StatusCode)}
		return nil, fmt.Errorf("azure voices request returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		failure = []zap.Field{logger.String("reason", "read_error")}
		return nil, fmt.Errorf("read azure voices response: %w", err)
	}
	if len(body) > maxResponseBytes {
		failure = []zap.Field{logger.String("reason", "response_too_large")}
		return nil, errors.New("azure voices response is too large")
	}
	var raw []azureVoice
	if err := json.Unmarshal(body, &raw); err != nil {
		failure = []zap.Field{logger.String("reason", "decode_error")}
		return nil, fmt.Errorf("decode azure voices response: %w", err)
	}
	voices = make([]domain.ProviderVoice, 0, len(raw))
	for _, v := range raw {
		if !isStandardNeural(v) {
			continue
		}
		name := v.DisplayName
		if name == "" {
			name = v.LocalName
		}
		voices = append(voices, domain.ProviderVoice{ProviderVoiceID: v.ShortName, Name: name, Locale: v.Locale, Gender: v.Gender})
	}
	return voices, nil
}

// transportReason classifies a failed request that never produced a response.
func transportReason(err error) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	}
	return "unreachable"
}

// statusReason classifies a non-200 answer by its HTTP status class.
func statusReason(code int) string {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return "auth_rejected"
	case code == http.StatusTooManyRequests:
		return "rate_limited"
	case code >= 500:
		return "http_5xx"
	}
	return "http_4xx"
}
