package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/converter"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func observedContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

func loggedText(logs *observer.ObservedLogs) string {
	var out strings.Builder
	for _, entry := range logs.All() {
		fmt.Fprintf(&out, "%s %v\n", entry.Message, entry.ContextMap())
	}
	return out.String()
}

func newLoggedConversion(t *testing.T, repo *creationRepository, store *conversionStore) *AvatarConversionService {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "asset-*.vrm")
	if err != nil {
		t.Fatal(err)
	}
	return &AvatarConversionService{
		repo: repo, store: store, runner: &conversionRunner{asset: &converter.Asset{File: file}},
		config: config.AvatarConversionConfig{Enabled: true, MaxInputBytes: 50 << 20},
	}
}

func TestAvatarCreationLogsStepsWithoutNameOrPersonality(t *testing.T) {
	ctx, logs := observedContext()
	source := avatarSource(t)
	svc := newLoggedConversion(t, &creationRepository{}, &conversionStore{objects: map[string]bool{}})
	user := inventoryUser()
	if _, err := svc.Create(ctx, user, bytes.NewReader(source), int64(len(source)),
		domain.CreateAvatarInput{Name: "SENTINEL-NAME-7f3a", Personality: "SENTINEL-PERSONALITY-7f3a"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	created := logs.FilterMessage("avatar created").All()
	if len(created) != 1 || created[0].ContextMap()["op"] != "inventory.avatar_create" ||
		created[0].ContextMap()["user_id"] != user.ID.String() || created[0].ContextMap()["bytes"] != int64(len(source)) {
		t.Fatalf("avatar created = %v", created)
	}
	step := logs.FilterMessage("step ok").All()
	if len(step) != 1 || step[0].ContextMap()["op"] != "inventory.avatar_convert" {
		t.Fatalf("conversion step = %v", step)
	}
	if text := loggedText(logs); strings.Contains(text, "SENTINEL") {
		t.Fatal("avatar name or personality reached the log")
	}
}

func TestAvatarCleanupFailureIsLogged(t *testing.T) {
	ctx, logs := observedContext()
	source := avatarSource(t)
	repo := &creationRepository{createErr: errors.New("database unavailable")}
	store := &conversionStore{objects: map[string]bool{}, deleteErr: errors.New("storage unavailable")}
	svc := newLoggedConversion(t, repo, store)
	if _, err := svc.Create(ctx, inventoryUser(), bytes.NewReader(source), int64(len(source)),
		domain.CreateAvatarInput{Name: "Test", Personality: "Friendly"}); err == nil {
		t.Fatal("Create must fail when the row cannot be written")
	}
	entries := logs.FilterMessage("unable to delete unreferenced avatar object").All()
	if len(entries) != 1 || entries[0].Level != zapcore.ErrorLevel || entries[0].ContextMap()["key"] != "converted.vrm" {
		t.Fatalf("cleanup failure log = %v", entries)
	}
	if n := logs.FilterMessage("avatar created").Len(); n != 0 {
		t.Fatalf("a failed creation logged %d created events", n)
	}
}
