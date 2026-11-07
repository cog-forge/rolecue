package service

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/profile/domain"
)

func TestProfileImageHostAllowlist(t *testing.T) {
	hosts := []string{"lh3.googleusercontent.com", "*.cdn.example.test", "Storage.Example.Test"}
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://lh3.googleusercontent.com/a/photo=s96", true},
		{"https://LH3.googleusercontent.com:443/a", true},
		{"https://storage.example.test/avatar.png", true},
		{"https://img.cdn.example.test/x.png", true},
		{"https://cdn.example.test/x.png", false},
		{"https://lh3.googleusercontent.com.evil.test/a", false},
		{"https://evil.test/lh3.googleusercontent.com", false},
		{"https://tracker.example.com/pixel.gif", false},
		{"http://lh3.googleusercontent.com/a", false},
	} {
		if got := validURL(tc.url) && allowedImageHost(tc.url, hosts); got != tc.want {
			t.Errorf("%s: allowed=%v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestPatchImageRejectsUnlistedHost(t *testing.T) {
	bad, good := "https://tracker.example.com/pixel.gif", "https://avatars.githubusercontent.com/u/1"
	patch := domain.Patch{Image: domain.StringChange{Present: true, Value: &bad}}
	if err := validatePatch(&patch, "candidate", []string{"avatars.githubusercontent.com"}); err == nil {
		t.Fatal("image on an unlisted host was accepted")
	}
	patch = domain.Patch{Image: domain.StringChange{Present: true, Value: &good}}
	if err := validatePatch(&patch, "candidate", []string{"avatars.githubusercontent.com"}); err != nil {
		t.Fatal(err)
	}
}
