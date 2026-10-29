package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
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
}

func (a *Azure) ListVoices(ctx context.Context) ([]domain.ProviderVoice, error) {
	if a.key == "" || a.baseURL == "" {
		return nil, service.ErrProviderNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/cognitiveservices/voices/list", nil)
	if err != nil {
		return nil, fmt.Errorf("build azure voices request: %w", err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", a.key)
	req.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		// The error carries the URL but never the key, which travels in a header.
		return nil, fmt.Errorf("azure voices request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("azure voices request returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read azure voices response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("azure voices response is too large")
	}
	var raw []azureVoice
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode azure voices response: %w", err)
	}
	voices := make([]domain.ProviderVoice, 0, len(raw))
	for _, v := range raw {
		name := v.DisplayName
		if name == "" {
			name = v.LocalName
		}
		voices = append(voices, domain.ProviderVoice{ProviderVoiceID: v.ShortName, Name: name, Locale: v.Locale, Gender: v.Gender})
	}
	return voices, nil
}
