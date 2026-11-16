package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The storage key of a model never leaves the service: not in the avatar DTOs, whichever way they are encoded.
func TestAvatarDTOsNeverCarryTheStorageKey(t *testing.T) {
	const key = "avatars/secret-object-key.vrm"
	stored := StoredAvatar{Avatar: Avatar{ID: uuid.New(), Name: "Alex", Personality: "calm"}, ModelPath: key}
	for name, value := range map[string]any{"stored": stored, "detail": AvatarDetail{Avatar: stored.Avatar}, "avatar": stored.Avatar} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), key) || strings.Contains(string(raw), "model_path") || strings.Contains(string(raw), "asset_id") {
			t.Fatalf("%s DTO leaks storage data: %s", name, raw)
		}
	}
}
