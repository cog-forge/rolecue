package runtime

import (
	"fmt"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/google/uuid"
)

func TestStreamRemembersTheLatestCommandIDs(t *testing.T) {
	stream := &sessionStream{seen: map[string]struct{}{}}
	for i := 0; i <= rememberedCommandIDs; i++ {
		stream.accept(fmt.Sprintf("id-%d", i))
	}
	if stream.accepted("id-0") {
		t.Fatal("the oldest id was not forgotten")
	}
	if !stream.accepted("id-1") || !stream.accepted(fmt.Sprintf("id-%d", rememberedCommandIDs)) {
		t.Fatal("a recent id was forgotten")
	}
}

func TestHubKeepsAStreamForReconnectsThenDropsIt(t *testing.T) {
	hub := NewHub()
	now := time.Now()
	hub.now = func() time.Time { return now }
	session := uuid.New()

	first, err := hub.attach(session)
	if err != nil || first.resumed {
		t.Fatalf("first attach resumed=%v err=%v", first.resumed, err)
	}
	first.stream.nextSeq()
	hub.release(first)

	now = now.Add(streamRetention - time.Second)
	second, _ := hub.attach(session)
	if second.stream != first.stream || !second.resumed || second.stream.nextSeq() != 2 {
		t.Fatal("a reconnect inside the retention did not keep the stream")
	}
	hub.release(second)

	now = now.Add(streamRetention + time.Second)
	third, _ := hub.attach(uuid.New())
	hub.release(third)
	fourth, _ := hub.attach(session)
	if fourth.stream == first.stream || fourth.resumed {
		t.Fatal("an expired stream was reused")
	}
	hub.release(fourth)
}

func TestSynthesisCacheEvictsTheLeastRecent(t *testing.T) {
	cache := newSynthesisCache(2)
	result := tts.Result{Audio: []byte("a"), MimeType: "audio/mpeg"}
	cache.put("v", "one", result)
	cache.put("v", "two", result)
	cache.get("v", "one")
	cache.put("v", "three", result)
	if _, ok := cache.get("v", "two"); ok {
		t.Fatal("the least recent entry was kept")
	}
	if _, ok := cache.get("v", "one"); !ok {
		t.Fatal("a recently used entry was evicted")
	}
	if newSynthesisCache(0) != nil {
		t.Fatal("a zero-size cache is not disabled")
	}
	var disabled *synthesisCache
	disabled.put("v", "one", result)
	if _, ok := disabled.get("v", "one"); ok {
		t.Fatal("a disabled cache returned an entry")
	}
}

func TestAudioTypesAndContainers(t *testing.T) {
	for input, want := range map[string]string{
		"audio/webm;codecs=opus": "audio/webm", "audio/ogg; codecs=opus": "audio/ogg", "audio/mp4": "audio/mp4",
		"audio/x-wav": "audio/wav", "AUDIO/WEBM": "audio/webm",
	} {
		if got, ok := normalizeAudioType(input); !ok || got != want {
			t.Errorf("normalizeAudioType(%q) = %q, %v", input, got, ok)
		}
	}
	for _, input := range []string{"", "audio/flac", "video/webm", "text/plain"} {
		if _, ok := normalizeAudioType(input); ok {
			t.Errorf("normalizeAudioType(%q) accepted", input)
		}
	}
	wav := append([]byte("RIFF\x00\x00\x00\x00WAVE"), 0)
	mp4 := []byte("\x00\x00\x00\x18ftypM4A ")
	for mediaType, data := range map[string][]byte{"audio/webm": webm("x"), "audio/ogg": []byte("OggS"), "audio/wav": wav, "audio/mp4": mp4} {
		if !matchesContainer(mediaType, data) {
			t.Errorf("%s rejected its own container", mediaType)
		}
		if matchesContainer(mediaType, []byte("garbage-bytes")) {
			t.Errorf("%s accepted garbage", mediaType)
		}
	}
}

func TestRoundWeightsKeepsThreeDecimals(t *testing.T) {
	got := roundWeights([]float32{0, 0.12345, 0.9999, 1})
	want := []float64{0, 0.123, 1, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("roundWeights = %v, want %v", got, want)
		}
	}
}
