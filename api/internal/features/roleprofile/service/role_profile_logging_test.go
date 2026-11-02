package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	roleprofile "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	jdSentinel       = "SENTINEL-JD-TEXT-4211"
	evidenceSentinel = "SENTINEL-EVIDENCE-9911"
	logJDText        = "Backend engineer using Go and PostgreSQL. "
)

func observedContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

// renderLogs flattens every message and field so a test can search all logged text.
func renderLogs(logs *observer.ObservedLogs) string {
	var b strings.Builder
	for _, entry := range logs.All() {
		fmt.Fprintf(&b, "%s %v\n", entry.Message, entry.ContextMap())
	}
	return b.String()
}

// logField returns one logged field as text, or fails when the entry does not exist.
func logField(t *testing.T, logs *observer.ObservedLogs, message string) (observer.LoggedEntry, map[string]any) {
	t.Helper()
	entries := logs.FilterMessage(message).All()
	if len(entries) == 0 {
		t.Fatalf("no log entry %q in %s", message, renderLogs(logs))
	}
	return entries[0], entries[0].ContextMap()
}

func analysisProfile(raw string) *profileFake {
	return &profileFake{profile: roleprofile.RoleProfile{ID: uuid.New(), JDID: uuid.New(), RawText: raw, Status: roleprofile.Draft}}
}

func newAnalysis(t *testing.T, repo RoleProfileRepository, extract Extractor, retries int) *RoleProfileApplication {
	t.Helper()
	extraction, err := NewExtractionService(extract, retries)
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewRoleProfileApplication(extraction, repo, &bankFake{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func candidateWithEvidence(name, evidence string) roleprofile.ExtractionCandidate {
	return roleprofile.ExtractionCandidate{Title: "Backend engineer", Skills: []roleprofile.ExtractedSkill{{Name: name, Category: roleprofile.ProgrammingLanguage, EvidenceText: &evidence}},
		Technologies: []string{}, DomainKnowledge: []string{}}
}

func TestAnalyzeLogsRootCauseWhenEvidenceRejected(t *testing.T) {
	ctx, logs := observedContext()
	repo := analysisProfile(strings.Repeat(logJDText, 4))
	app := newAnalysis(t, repo, extractorFunc(func(context.Context, string) (roleprofile.ExtractionCandidate, error) {
		return candidateWithEvidence("Rust", "10 years of Rust"), nil
	}), 0)

	_, err := app.Analyze(ctx, uuid.New(), "candidate", repo.profile.ID)
	assertCode(t, err, apperror.CodeInvalidExtractionOutput)

	entry, fields := logField(t, logs, "step failed")
	if entry.Level != zap.WarnLevel {
		t.Fatalf("level = %s, want warn", entry.Level)
	}
	if fields["op"] != "roleprofile.analyze" || fields["gate"] != "evidence" || fields["profile_id"] != repo.profile.ID.String() {
		t.Fatalf("fields = %v", fields)
	}
	cause := fmt.Sprint(fields["cause_chain"])
	for _, want := range []string{"Requirement evidence must be an excerpt", "evidence for requirement 0 not found in source text"} {
		if !strings.Contains(cause, want) {
			t.Fatalf("cause_chain %s does not name %q", cause, want)
		}
	}
	if !repo.released {
		t.Fatal("analysis was not released after the rejected evidence")
	}
}

func TestAnalyzeWarnsWhenReleaseFails(t *testing.T) {
	ctx, logs := observedContext()
	fake := analysisProfile(strings.Repeat(logJDText, 4))
	repo := releaseFailingRepo{fake}
	app := newAnalysis(t, repo, extractorFunc(func(context.Context, string) (roleprofile.ExtractionCandidate, error) {
		return candidateWithEvidence("Rust", "10 years of Rust"), nil
	}), 0)

	_, err := app.Analyze(ctx, uuid.New(), "candidate", fake.profile.ID)
	assertCode(t, err, apperror.CodeInvalidExtractionOutput)

	_, fields := logField(t, logs, "role profile analysis release failed")
	if fields["profile_id"] != fake.profile.ID.String() || !strings.Contains(fmt.Sprint(fields["cause_chain"]), "connection reset") {
		t.Fatalf("release warning fields = %v", fields)
	}
}

type releaseFailingRepo struct{ *profileFake }

func (releaseFailingRepo) ReleaseAnalysis(context.Context, uuid.UUID) error {
	return errors.New("connection reset")
}

func TestExtractionLogsEachAttemptAndRejectionStage(t *testing.T) {
	ctx, logs := observedContext()
	calls := 0
	extract := extractorFunc(func(context.Context, string) (roleprofile.ExtractionCandidate, error) {
		calls++
		if calls == 1 {
			// An empty title is a validation rejection, which the shared retry budget allows once.
			return roleprofile.ExtractionCandidate{Title: " ", Technologies: []string{"Go"}, Skills: []roleprofile.ExtractedSkill{}, DomainKnowledge: []string{}}, nil
		}
		return roleprofile.ExtractionCandidate{Title: "Backend engineer", Technologies: []string{"Go"}, Skills: []roleprofile.ExtractedSkill{}, DomainKnowledge: []string{}}, nil
	})
	service, err := NewExtractionService(extract, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Extract(ctx, strings.Repeat(logJDText, 4)); err != nil {
		t.Fatal(err)
	}

	_, rejected := logField(t, logs, "extraction attempt rejected, retrying")
	if rejected["attempt"] != int64(1) || rejected["stage"] != "validation" || rejected["will_retry"] != true {
		t.Fatalf("rejection fields = %v", rejected)
	}
	if !strings.Contains(fmt.Sprint(rejected["cause_chain"]), "title is required") {
		t.Fatalf("rejection cause does not name the rule: %v", rejected["cause_chain"])
	}
	_, accepted := logField(t, logs, "extraction attempt accepted")
	if accepted["attempt"] != int64(2) {
		t.Fatalf("accepted fields = %v", accepted)
	}
}

func TestAnalysisLogsNeverContainDocumentText(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx, logs := observedContext()
		repo := analysisProfile(strings.Repeat(logJDText, 4) + jdSentinel)
		app := newAnalysis(t, repo, extractorFunc(func(context.Context, string) (roleprofile.ExtractionCandidate, error) {
			return candidateWithEvidence("Go", "Backend engineer using Go"), nil
		}), 0)
		if _, err := app.Analyze(ctx, uuid.New(), "candidate", repo.profile.ID); err != nil {
			t.Fatal(err)
		}
		logField(t, logs, "step ok")
		if strings.Contains(renderLogs(logs), jdSentinel) {
			t.Fatalf("job description text reached the logs: %s", renderLogs(logs))
		}
	})
	t.Run("rejected evidence", func(t *testing.T) {
		ctx, logs := observedContext()
		repo := analysisProfile(strings.Repeat(logJDText, 4) + jdSentinel)
		app := newAnalysis(t, repo, extractorFunc(func(context.Context, string) (roleprofile.ExtractionCandidate, error) {
			return candidateWithEvidence("Rust", evidenceSentinel+" of Rust"), nil
		}), 0)
		if _, err := app.Analyze(ctx, uuid.New(), "candidate", repo.profile.ID); err == nil {
			t.Fatal("expected rejected evidence")
		}
		logField(t, logs, "step failed")
		if rendered := renderLogs(logs); strings.Contains(rendered, evidenceSentinel) || strings.Contains(rendered, jdSentinel) {
			t.Fatalf("document text reached the logs: %s", rendered)
		}
	})
}

func TestConfirmLogsReusedQuestionBank(t *testing.T) {
	ctx, logs := observedContext()
	app, repo, bank := fixture(t)
	bank.err = apperror.New(apperror.CodeQuestionBankExists, "exists")

	if _, err := app.Confirm(ctx, uuid.New(), "candidate", repo.profile.ID); err != nil {
		t.Fatal(err)
	}
	entry, fields := logField(t, logs, "practice question bank already exists, reusing it")
	if entry.Level != zap.InfoLevel || fields["jd_id"] != repo.profile.JDID.String() {
		t.Fatalf("reuse log = level %s fields %v", entry.Level, fields)
	}
}
