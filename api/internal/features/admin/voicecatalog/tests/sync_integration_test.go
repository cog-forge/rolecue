//go:build integration

package tests

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/provider"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// isolated reports a per-test provider name, so tests never touch each other's rows or real ones.
type isolated struct {
	*provider.Fake
	name string
}

func (p isolated) Name() string { return p.name }

// syncFixture builds the real repository and service over the disposable database with a fake
// provider. Rows synced under the provider name are deleted at the end, after referencing rows.
func syncFixture(t *testing.T) (*fixture, *service.Service, *repository.Repository, isolated) {
	t.Helper()
	f := open(t)
	name := "it-" + f.tag
	f.later(`DELETE FROM public.voices WHERE provider = $1`, name)
	repo := repository.NewRepository(f.pool)
	fake := isolated{Fake: provider.NewFake(), name: name}
	svc, err := service.New(repo, fake, []string{"en"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return f, svc, repo, fake
}

func pv(id, name, locale, gender string) domain.ProviderVoice {
	return domain.ProviderVoice{ProviderVoiceID: id, Name: name, Locale: locale, Gender: gender}
}

func (f *fixture) sync(svc *service.Service) (domain.VoiceSyncResult, error) {
	return svc.Sync(f.ctx, uuid.New(), "admin")
}

// byProviderID reads one synced voice straight from the table.
func (f *fixture) byProviderID(providerName, id string) (name string, active bool, found bool) {
	rows, err := f.pool.Query(f.ctx, `SELECT name, is_active FROM public.voices WHERE provider = $1 AND provider_voice_id = $2`, providerName, id)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&name, &active); err != nil {
			f.t.Fatal(err)
		}
		found = true
	}
	return name, active, found
}

func (f *fixture) countProvider(providerName string) int {
	var n int
	if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM public.voices WHERE provider = $1`, providerName).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func TestSyncUpsertsDeactivatesAndRepeats(t *testing.T) {
	f, svc, repo, fake := syncFixture(t)
	legacy := f.voice("legacy", true)
	legacyOff := f.voice("legacy off", false)

	fake.Set(pv("en-US-AvaNeural", "Ava", "en-US", "Female"), pv("en-US-AndrewNeural", "Andrew", "en-US", "Male"),
		pv("fr-FR-DeniseNeural", "Denise", "fr-FR", "Female"))
	got, err := f.sync(svc)
	if err != nil || got.Added != 2 || got.Updated != 0 || got.Deactivated != 0 || got.Unchanged != 0 || got.Fetched != 2 {
		t.Fatalf("first sync = %+v, %v (the French voice is outside the locale filter)", got, err)
	}
	if _, _, found := f.byProviderID(fake.name, "fr-FR-DeniseNeural"); found {
		t.Fatal("a voice outside the sync locales was stored")
	}

	again, err := f.sync(svc)
	if err != nil || again.Added+again.Updated+again.Deactivated != 0 || again.Unchanged != 2 {
		t.Fatalf("repeat sync = %+v, %v", again, err)
	}

	// Ava is renamed, Andrew disappears, Sonia is new.
	fake.Set(pv("en-US-AvaNeural", "Ava Multilingual", "en-US", "Female"), pv("en-GB-SoniaNeural", "Sonia", "en-GB", "Female"))
	got, err = f.sync(svc)
	if err != nil || got.Added != 1 || got.Updated != 1 || got.Deactivated != 1 || got.Unchanged != 0 {
		t.Fatalf("change sync = %+v, %v", got, err)
	}
	if name, active, _ := f.byProviderID(fake.name, "en-US-AvaNeural"); name != "Ava Multilingual" || !active {
		t.Fatalf("renamed voice = %q active=%v", name, active)
	}
	if _, active, found := f.byProviderID(fake.name, "en-US-AndrewNeural"); !found || active {
		t.Fatalf("dropped voice must stay as an inactive row, found=%v active=%v", found, active)
	}

	// The provider offers Andrew again: the row is reused, not duplicated, and stays as an Admin left it.
	fake.Set(pv("en-US-AvaNeural", "Ava Multilingual", "en-US", "Female"), pv("en-GB-SoniaNeural", "Sonia", "en-GB", "Female"),
		pv("en-US-AndrewNeural", "Andrew", "en-US", "Male"))
	if got, err = f.sync(svc); err != nil || got.Added != 0 || got.Updated != 0 || got.Unchanged != 3 {
		t.Fatalf("return sync = %+v, %v", got, err)
	}
	if f.countProvider(fake.name) != 3 {
		t.Fatalf("provider has %d rows, want 3", f.countProvider(fake.name))
	}
	if _, active, _ := f.byProviderID(fake.name, "en-US-AndrewNeural"); active {
		t.Fatal("a sync must not reactivate a deactivated voice")
	}

	for id, wantActive := range map[uuid.UUID]bool{legacy: true, legacyOff: false} {
		v, err := repo.Get(f.ctx, id)
		if err != nil || v.IsActive != wantActive || v.Provider != nil || v.ProviderVoiceID != nil || v.Locale != nil || v.Gender != nil {
			t.Fatalf("legacy voice changed: %+v, %v", v, err)
		}
	}
}

func TestSyncedVoicesExposeProviderMetadata(t *testing.T) {
	f, svc, repo, fake := syncFixture(t)
	fake.Set(pv("en-US-AvaNeural", "Ava", "en-US", "Female"), pv("en-US-NoGender", "No Gender", "en-US", ""))
	if _, err := f.sync(svc); err != nil {
		t.Fatal(err)
	}
	rows, total, err := repo.List(f.ctx, domain.VoiceListFilter{Status: domain.VoiceStatusAll, Query: "Ava", Limit: 100})
	if err != nil || total == 0 {
		t.Fatalf("list = %+v, %d, %v", rows, total, err)
	}
	var ava *domain.VoiceProfile
	for i := range rows {
		if rows[i].ProviderVoiceID != nil && *rows[i].ProviderVoiceID == "en-US-AvaNeural" && *rows[i].Provider == fake.name {
			ava = &rows[i]
		}
	}
	if ava == nil || *ava.Locale != "en-US" || *ava.Gender != "Female" || !ava.IsActive {
		t.Fatalf("metadata missing from the list: %+v", rows)
	}
	var id uuid.UUID
	if err := f.pool.QueryRow(f.ctx, `SELECT id FROM public.voices WHERE provider = $1 AND provider_voice_id = 'en-US-NoGender'`, fake.name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if v, err := repo.Get(f.ctx, id); err != nil || v.Gender != nil || *v.Locale != "en-US" {
		t.Fatalf("an empty gender should be stored as null: %+v, %v", v, err)
	}
}

func TestSyncKeepsHistoryOfReferencedVoices(t *testing.T) {
	f, svc, repo, fake := syncFixture(t)
	fake.Set(pv("en-US-AvaNeural", "Ava", "en-US", "Female"), pv("en-US-AndrewNeural", "Andrew", "en-US", "Male"))
	if _, err := f.sync(svc); err != nil {
		t.Fatal(err)
	}
	var ava uuid.UUID
	if err := f.pool.QueryRow(f.ctx, `SELECT id FROM public.voices WHERE provider = $1 AND provider_voice_id = 'en-US-AvaNeural'`, fake.name).Scan(&ava); err != nil {
		t.Fatal(err)
	}
	f.posting(ava)
	f.interview(ava)

	fake.Set(pv("en-US-AndrewNeural", "Andrew", "en-US", "Male"))
	got, err := f.sync(svc)
	if err != nil || got.Deactivated != 1 {
		t.Fatalf("sync = %+v, %v", got, err)
	}
	v, err := repo.Get(f.ctx, ava)
	if err != nil || v.IsActive || v.InterviewCount != 1 || v.JobPostingCount != 1 || v.Name != "Ava" {
		t.Fatalf("the referenced voice must survive deactivated with its references: %+v, %v", v, err)
	}
	var refs int
	if err := f.pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.interviews WHERE voice_id = $1) + (SELECT count(*) FROM public.job_postings WHERE required_voice_id = $1)`, ava).Scan(&refs); err != nil || refs != 2 {
		t.Fatalf("references = %d, %v", refs, err)
	}

	// Lock-before-delete still applies to synced voices: referenced ones are deactivated, unused deleted.
	if outcome, err := repo.Remove(f.ctx, ava); err != nil || outcome != domain.VoiceDeactivated {
		t.Fatalf("remove referenced synced voice: %q, %v", outcome, err)
	}
	var andrew uuid.UUID
	if err := f.pool.QueryRow(f.ctx, `SELECT id FROM public.voices WHERE provider = $1 AND provider_voice_id = 'en-US-AndrewNeural'`, fake.name).Scan(&andrew); err != nil {
		t.Fatal(err)
	}
	if outcome, err := repo.Remove(f.ctx, andrew); err != nil || outcome != domain.VoiceDeleted {
		t.Fatalf("remove unused synced voice: %q, %v", outcome, err)
	}
	// After deletion the provider's next sync simply adds it again.
	if got, err = f.sync(svc); err != nil || got.Added != 1 {
		t.Fatalf("sync after delete = %+v, %v", got, err)
	}
}

func TestSyncFailureLeavesTheCatalogUnchanged(t *testing.T) {
	f, svc, _, fake := syncFixture(t)
	fake.Set(pv("en-US-AvaNeural", "Ava", "en-US", "Female"), pv("en-US-AndrewNeural", "Andrew", "en-US", "Male"))
	if _, err := f.sync(svc); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		rows, err := f.pool.Query(f.ctx, `SELECT provider_voice_id, name, locale, coalesce(gender, '-'), is_active FROM public.voices WHERE provider = $1 ORDER BY provider_voice_id`, fake.name)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id, name, locale, gender string
			var active bool
			if err := rows.Scan(&id, &name, &locale, &gender, &active); err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprint(id, name, locale, gender, active))
		}
		return strings.Join(out, ";")
	}
	before := snapshot()

	// Provider failure and invalid answers never reach the database.
	fake.Fail(fmt.Errorf("provider down"))
	_, err := f.sync(svc)
	wantCode(t, err, apperror.CodeVoiceSyncFailed)
	fake.Set(pv("en-US-NewNeural", "New", "en-US", "Male"), pv("", "Broken", "en-US", "Male"))
	_, err = f.sync(svc)
	wantCode(t, err, apperror.CodeVoiceSyncFailed)
	fake.Set()
	_, err = f.sync(svc)
	wantCode(t, err, apperror.CodeVoiceSyncFailed)
	if snapshot() != before {
		t.Fatal("a failed fetch changed the catalog")
	}

	// A database error half-way (the deactivation step fails after the upsert succeeded) rolls the whole
	// transaction back: the new voice is not left behind and the dropped voice stays active.
	fn := "vc_block_" + strings.ReplaceAll(f.tag, "-", "_")
	f.later(`DROP FUNCTION IF EXISTS public.` + fn + `()`)
	f.later(`DROP TRIGGER IF EXISTS ` + fn + ` ON public.voices`)
	f.exec(`CREATE FUNCTION public.` + fn + `() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
		IF NEW.provider = '` + fake.name + `' AND NOT NEW.is_active AND OLD.is_active THEN RAISE EXCEPTION 'blocked by test'; END IF; RETURN NEW; END $$`)
	f.exec(`CREATE TRIGGER ` + fn + ` BEFORE UPDATE ON public.voices FOR EACH ROW EXECUTE FUNCTION public.` + fn + `()`)
	fake.Set(pv("en-US-AvaNeural", "Ava Renamed", "en-US", "Female"), pv("en-US-NewNeural", "New", "en-US", "Male"))
	_, err = f.sync(svc)
	wantCode(t, err, apperror.CodeInternal)
	if snapshot() != before {
		t.Fatalf("a database failure left a partial sync behind:\n%s\nwant\n%s", snapshot(), before)
	}
}

func TestConcurrentSyncsAgree(t *testing.T) {
	f, svc, _, fake := syncFixture(t)
	fake.Set(pv("en-US-AvaNeural", "Ava", "en-US", "Female"), pv("en-US-AndrewNeural", "Andrew", "en-US", "Male"), pv("en-US-BrianNeural", "Brian", "en-US", "Male"))

	var wg sync.WaitGroup
	results := make([]domain.VoiceSyncResult, 5)
	errs := make([]error, len(results))
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = svc.Sync(context.Background(), uuid.New(), "admin")
		}()
	}
	wg.Wait()
	added := 0
	for i, err := range errs {
		if err != nil {
			t.Fatalf("sync %d: %v", i, err)
		}
		added += results[i].Added
	}
	if added != 3 || f.countProvider(fake.name) != 3 {
		t.Fatalf("added %d rows in total, %d stored; want exactly 3 and 3", added, f.countProvider(fake.name))
	}
}

func TestProviderColumnsStayConsistent(t *testing.T) {
	f := open(t)
	name := "it-" + f.tag
	f.later(`DELETE FROM public.voices WHERE provider = $1`, name)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO public.voices (name, provider) VALUES ('half', $1)`, name); err == nil {
		t.Fatal("a provider without a provider voice id must be rejected")
	}
	f.exec(`INSERT INTO public.voices (name, provider, provider_voice_id) VALUES ('one', $1, 'same')`, name)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO public.voices (name, provider, provider_voice_id) VALUES ('two', $1, 'same')`, name); err == nil {
		t.Fatal("the same provider voice id must be unique per provider")
	}
	f.exec(`INSERT INTO public.voices (name, provider, provider_voice_id) VALUES ('other provider', $1, 'same')`, name+"-b")
	f.later(`DELETE FROM public.voices WHERE provider = $1`, name+"-b")
	f.exec(`INSERT INTO public.voices (name) VALUES ($1), ($1)`, f.tag+" legacy twin")
	f.later(`DELETE FROM public.voices WHERE name = $1`, f.tag+" legacy twin")
}
