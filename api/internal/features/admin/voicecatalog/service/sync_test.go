package service_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/provider"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type row struct {
	provider, providerVoiceID *string
	name, locale, gender      string
	active                    bool
}

// memoryCatalog applies a sync the way the SQL does, all or nothing: it upserts by provider voice ID
// without touching is_active, deactivates the provider's voices missing from the list, and ignores
// legacy rows (no provider).
type memoryCatalog struct {
	rows    []*row
	failOn  error
	syncs   int
	lastIDs []string
}

func ptr(s string) *string { return &s }

func (m *memoryCatalog) legacy(name string, active bool) {
	m.rows = append(m.rows, &row{name: name, active: active})
}

func (m *memoryCatalog) provided(provider, id, name string, active bool) {
	m.rows = append(m.rows, &row{provider: ptr(provider), providerVoiceID: ptr(id), name: name, locale: "en-US", gender: "Female", active: active})
}

// snapshot is a stable text form of the catalog for before/after comparisons.
func (m *memoryCatalog) snapshot() string {
	lines := make([]string, 0, len(m.rows))
	for _, r := range m.rows {
		p, id := "-", "-"
		if r.provider != nil {
			p, id = *r.provider, *r.providerVoiceID
		}
		active := "off"
		if r.active {
			active = "on"
		}
		lines = append(lines, strings.Join([]string{p, id, r.name, r.locale, r.gender, active}, "|"))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (m *memoryCatalog) find(provider, id string) *row {
	for _, r := range m.rows {
		if r.provider != nil && *r.provider == provider && *r.providerVoiceID == id {
			return r
		}
	}
	return nil
}

func (m *memoryCatalog) Sync(_ context.Context, name string, voices []domain.ProviderVoice) (domain.VoiceSyncResult, error) {
	m.syncs++
	if m.failOn != nil {
		return domain.VoiceSyncResult{}, m.failOn
	}
	result := domain.VoiceSyncResult{Provider: name, Fetched: len(voices)}
	offered := map[string]bool{}
	m.lastIDs = m.lastIDs[:0]
	for _, v := range voices {
		offered[v.ProviderVoiceID] = true
		m.lastIDs = append(m.lastIDs, v.ProviderVoiceID)
		existing := m.find(name, v.ProviderVoiceID)
		switch {
		case existing == nil:
			m.rows = append(m.rows, &row{provider: ptr(name), providerVoiceID: ptr(v.ProviderVoiceID), name: v.Name, locale: v.Locale, gender: v.Gender, active: true})
			result.Added++
		case existing.name != v.Name || existing.locale != v.Locale || existing.gender != v.Gender:
			existing.name, existing.locale, existing.gender = v.Name, v.Locale, v.Gender
			result.Updated++
		}
	}
	for _, r := range m.rows {
		if r.provider != nil && *r.provider == name && r.active && !offered[*r.providerVoiceID] {
			r.active = false
			result.Deactivated++
		}
	}
	result.Unchanged = result.Fetched - result.Added - result.Updated
	return result, nil
}

func (m *memoryCatalog) List(context.Context, domain.VoiceListFilter) ([]domain.VoiceProfile, int64, error) {
	return nil, 0, nil
}
func (m *memoryCatalog) Get(context.Context, uuid.UUID) (domain.VoiceProfile, error) {
	return domain.VoiceProfile{}, domain.ErrVoiceNotFound
}
func (m *memoryCatalog) Remove(context.Context, uuid.UUID) (domain.VoiceDeleteOutcome, error) {
	return "", domain.ErrVoiceNotFound
}
func (m *memoryCatalog) Activate(context.Context, uuid.UUID) error { return domain.ErrVoiceNotFound }

func pv(id, name, locale string) domain.ProviderVoice {
	return domain.ProviderVoice{ProviderVoiceID: id, Name: name, Locale: locale, Gender: "Female"}
}

func newSyncService(t *testing.T, catalog *memoryCatalog, voices *provider.Fake, locales ...string) *service.Service {
	t.Helper()
	svc, err := service.New(catalog, voices, locales, nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func syncAs(svc *service.Service, role string) (domain.VoiceSyncResult, error) {
	return svc.Sync(context.Background(), uuid.New(), role)
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var got *apperror.AppError
	if !errors.As(err, &got) || got.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestSyncAddsUpdatesAndCounts(t *testing.T) {
	catalog := &memoryCatalog{}
	catalog.provided(provider.FakeName, "en-US-AvaNeural", "Old Ava", true)
	catalog.provided(provider.FakeName, "en-US-AndrewNeural", "Andrew", true)
	voices := provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US"), pv("en-US-AndrewNeural", "Andrew", "en-US"), pv("en-GB-SoniaNeural", "Sonia", "en-GB"))

	got, err := syncAs(newSyncService(t, catalog, voices, "en"), "admin")
	if err != nil {
		t.Fatal(err)
	}
	want := domain.VoiceSyncResult{Provider: provider.FakeName, Fetched: 3, Added: 1, Updated: 1, Deactivated: 0, Unchanged: 1}
	if got != want {
		t.Fatalf("result = %+v, want %+v", got, want)
	}
	if r := catalog.find(provider.FakeName, "en-US-AvaNeural"); r.name != "Ava" {
		t.Fatalf("name = %q, want the provider's", r.name)
	}
}

func TestSyncDeactivatesVoicesTheProviderDroppedAndNeverDeletes(t *testing.T) {
	catalog := &memoryCatalog{}
	catalog.provided(provider.FakeName, "en-US-AvaNeural", "Ava", true)
	catalog.provided(provider.FakeName, "en-US-GoneNeural", "Gone", true)
	catalog.provided(provider.FakeName, "en-US-AlreadyOff", "Off", false)
	voices := provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US"))

	got, err := syncAs(newSyncService(t, catalog, voices, "en"), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Deactivated != 1 || got.Unchanged != 1 {
		t.Fatalf("result = %+v, want 1 deactivated, 1 unchanged", got)
	}
	if len(catalog.rows) != 3 {
		t.Fatalf("catalog has %d rows, a sync must never delete", len(catalog.rows))
	}
	if r := catalog.find(provider.FakeName, "en-US-GoneNeural"); r == nil || r.active {
		t.Fatal("the dropped voice should stay as an inactive row")
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	catalog := &memoryCatalog{}
	catalog.provided(provider.FakeName, "en-US-GoneNeural", "Gone", true)
	voices := provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US"), pv("en-US-AndrewNeural", "Andrew", "en-US"))
	svc := newSyncService(t, catalog, voices, "en")

	first, err := syncAs(svc, "admin")
	if err != nil || first.Added != 2 || first.Deactivated != 1 {
		t.Fatalf("first sync = %+v, %v", first, err)
	}
	before := catalog.snapshot()
	second, err := syncAs(svc, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if second.Added+second.Updated+second.Deactivated != 0 || second.Unchanged != 2 {
		t.Fatalf("repeat sync = %+v, want only unchanged", second)
	}
	if catalog.snapshot() != before {
		t.Fatal("repeating a sync changed the catalog")
	}
}

func TestSyncKeepsLegacyRowsUntouched(t *testing.T) {
	catalog := &memoryCatalog{}
	catalog.legacy("Legacy Active", true)
	catalog.legacy("Legacy Inactive", false)
	voices := provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US"))

	got, err := syncAs(newSyncService(t, catalog, voices, "en"), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Added != 1 || got.Deactivated != 0 {
		t.Fatalf("result = %+v", got)
	}
	if !catalog.rows[0].active || catalog.rows[1].active || catalog.rows[0].provider != nil {
		t.Fatal("legacy rows (NULL provider) must keep their state")
	}
}

func TestSyncDoesNotReactivateAnAdminDeactivation(t *testing.T) {
	catalog := &memoryCatalog{}
	catalog.provided(provider.FakeName, "en-US-AvaNeural", "Ava", false)
	voices := provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US"))

	if _, err := syncAs(newSyncService(t, catalog, voices, "en"), "admin"); err != nil {
		t.Fatal(err)
	}
	if catalog.find(provider.FakeName, "en-US-AvaNeural").active {
		t.Fatal("a sync must not undo an Admin's deactivation")
	}
}

func TestSyncProviderFailureLeavesTheCatalogUnchanged(t *testing.T) {
	catalog := &memoryCatalog{}
	catalog.provided(provider.FakeName, "en-US-AvaNeural", "Ava", true)
	catalog.legacy("Legacy", true)
	before := catalog.snapshot()
	voices := provider.NewFake()
	voices.Fail(errors.New("connection reset"))

	_, err := syncAs(newSyncService(t, catalog, voices, "en"), "admin")
	wantCode(t, err, apperror.CodeVoiceSyncFailed)
	if catalog.syncs != 0 || catalog.snapshot() != before {
		t.Fatal("a provider failure must not write to the catalog")
	}
}

func TestSyncUnconfiguredProviderIsUnavailable(t *testing.T) {
	catalog := &memoryCatalog{}
	voices := provider.NewFake()
	voices.Fail(service.ErrProviderNotConfigured)

	_, err := syncAs(newSyncService(t, catalog, voices, "en"), "admin")
	wantCode(t, err, apperror.CodeVoiceProviderUnavailable)
}

func TestSyncRejectsInvalidOrEmptyResponsesBeforeWriting(t *testing.T) {
	long := strings.Repeat("x", 201)
	tests := map[string][]domain.ProviderVoice{
		"empty list":              {},
		"nothing in the locales":  {pv("fr-FR-DeniseNeural", "Denise", "fr-FR")},
		"missing id":              {pv("en-US-AvaNeural", "Ava", "en-US"), pv("", "Nameless", "en-US")},
		"missing name":            {pv("en-US-AvaNeural", " ", "en-US")},
		"missing locale":          {pv("en-US-AvaNeural", "Ava", "")},
		"duplicate id":            {pv("en-US-AvaNeural", "Ava", "en-US"), pv("en-US-AvaNeural", "Ava Again", "en-US")},
		"oversized name":          {pv("en-US-AvaNeural", long, "en-US")},
		"nul in id":               {pv("en-US-Ava\x00Neural", "Ava", "en-US")},
		"invalid utf-8":           {pv("en-US-Ava\xffNeural", "Ava", "en-US")},
		"bad entry outside scope": {pv("en-US-AvaNeural", "Ava", "en-US"), pv("", "Denise", "fr-FR")},
	}
	for name, response := range tests {
		t.Run(name, func(t *testing.T) {
			catalog := &memoryCatalog{}
			catalog.provided(provider.FakeName, "en-US-AvaNeural", "Ava", true)
			before := catalog.snapshot()

			_, err := syncAs(newSyncService(t, catalog, provider.NewFake(response...), "en"), "admin")
			wantCode(t, err, apperror.CodeVoiceSyncFailed)
			if catalog.syncs != 0 || catalog.snapshot() != before {
				t.Fatal("an invalid response must not reach or change the catalog")
			}
		})
	}
}

func TestSyncRepositoryFailureIsReturned(t *testing.T) {
	catalog := &memoryCatalog{failOn: apperror.New(apperror.CodeInternal, "voice catalog persistence failed")}
	_, err := syncAs(newSyncService(t, catalog, provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US")), "en"), "admin")
	wantCode(t, err, apperror.CodeInternal)
}

func TestSyncFiltersByConfiguredLocales(t *testing.T) {
	all := []domain.ProviderVoice{
		pv("en-US-AvaNeural", "Ava", "en-US"),
		pv("en-GB-SoniaNeural", "Sonia", "en-GB"),
		pv("fr-FR-DeniseNeural", "Denise", "fr-FR"),
		pv("en-ZA-LeahNeural", "Leah", "EN-za"),
		pv("ensemble-X", "Not english", "ensemble"),
	}
	tests := []struct {
		name    string
		locales []string
		want    []string
	}{
		{"language prefix", []string{"en"}, []string{"en-US-AvaNeural", "en-GB-SoniaNeural", "en-ZA-LeahNeural"}},
		{"exact locale", []string{"en-GB"}, []string{"en-GB-SoniaNeural"}},
		{"several, case and space insensitive", []string{" EN-us ", "fr"}, []string{"en-US-AvaNeural", "fr-FR-DeniseNeural"}},
		{"no filter keeps everything", nil, []string{"en-US-AvaNeural", "en-GB-SoniaNeural", "fr-FR-DeniseNeural", "en-ZA-LeahNeural", "ensemble-X"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := &memoryCatalog{}
			if _, err := syncAs(newSyncService(t, catalog, provider.NewFake(all...), tt.locales...), "admin"); err != nil {
				t.Fatal(err)
			}
			if strings.Join(catalog.lastIDs, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("synced %v, want %v", catalog.lastIDs, tt.want)
			}
		})
	}
}

func TestSyncDeactivatesVoicesLeftOutByNarrowerLocales(t *testing.T) {
	// A voice synced earlier under a wider filter is "no longer offered" once the filter narrows.
	catalog := &memoryCatalog{}
	catalog.provided(provider.FakeName, "fr-FR-DeniseNeural", "Denise", true)
	voices := provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US"), pv("fr-FR-DeniseNeural", "Denise", "fr-FR"))

	got, err := syncAs(newSyncService(t, catalog, voices, "en"), "admin")
	if err != nil || got.Deactivated != 1 {
		t.Fatalf("result = %+v, %v", got, err)
	}
}

func TestSyncOnlyAdminsAndNeverCallsTheProvider(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", ""} {
		voices := provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US"))
		catalog := &memoryCatalog{}
		_, err := syncAs(newSyncService(t, catalog, voices, "en"), role)
		wantCode(t, err, apperror.CodeForbidden)
		if voices.Calls() != 0 || catalog.syncs != 0 {
			t.Fatalf("role %q reached the provider or the catalog", role)
		}
	}
}

func TestSyncLogsTheSummaryWithoutSecrets(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	svc, err := service.New(&memoryCatalog{}, provider.NewFake(pv("en-US-AvaNeural", "Ava", "en-US")), []string{"en"}, &logger.Logger{Logger: zap.New(core)})
	if err != nil {
		t.Fatal(err)
	}
	admin := uuid.New()
	if _, err := svc.Sync(context.Background(), admin, "admin"); err != nil {
		t.Fatal(err)
	}
	entries := logs.FilterMessage("voice catalog synced").All()
	if len(entries) != 1 {
		t.Fatalf("got %d sync log entries, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["admin_id"] != admin.String() || fields["provider"] != provider.FakeName || fields["added"] != int64(1) {
		t.Fatalf("log fields = %v", fields)
	}
}
