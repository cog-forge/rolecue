package sdk

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

// batchJSON is one animation event; frame i has jawOpen = fill+i.
func batchJSON(t *testing.T, index, frames int, fill float32) string {
	t.Helper()
	poses := make([][]float32, frames)
	for i := range poses {
		poses[i] = make([]float32, tts.BlendShapeCount)
		poses[i][17] = fill + float32(i)
	}
	raw, err := json.Marshal(map[string]any{"FrameIndex": index, "BlendShapes": poses})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestCollectorOrdersTimelines(t *testing.T) {
	c := newCollector()
	// Arrival order as seen in the spike.
	c.viseme(500_000, 0, "")
	c.viseme(1_000_000, 19, "")
	c.viseme(9_812_500, 16, "")
	c.word(500_000, 212*time.Millisecond, "Tell", boundaryWord)
	c.word(2_750_000, 100*time.Millisecond, "me", boundaryWord)
	c.word(500_000, 4537*time.Millisecond, "Tell me.", boundarySentence)
	c.word(1_250_000, 50*time.Millisecond+400*time.Microsecond, " the ", boundaryWord)
	c.viseme(0, 0, batchJSON(t, 0, 2, 0))
	c.word(37_870_000, 100*time.Millisecond, ".", boundaryPunctuation)
	c.viseme(0, 0, batchJSON(t, 2, 1, 10))
	c.word(3_000_000, 0, "  ", boundaryWord)
	c.finished()

	got := c.result()

	wantVisemes := []tts.Viseme{{OffsetMs: 50, ID: 0}, {OffsetMs: 100, ID: 19}, {OffsetMs: 981, ID: 16}}
	if !reflect.DeepEqual(got.visemes, wantVisemes) {
		t.Fatalf("visemes = %+v", got.visemes)
	}
	wantWords := []tts.WordBoundary{
		{Text: "Tell", OffsetMs: 50, DurationMs: 212},
		{Text: "the", OffsetMs: 125, DurationMs: 50},
		{Text: "me", OffsetMs: 275, DurationMs: 100},
	}
	if !reflect.DeepEqual(got.words, wantWords) {
		t.Fatalf("words = %+v", got.words)
	}
	if len(got.blendShapes) != 3 || got.droppedFrames != 0 {
		t.Fatalf("frames = %d dropped = %d", len(got.blendShapes), got.droppedFrames)
	}
	for i, wantMs := range []int{0, 17, 33} {
		f := got.blendShapes[i]
		if f.OffsetMs != wantMs || len(f.Values) != tts.BlendShapeCount {
			t.Fatalf("frame %d = offset %d, %d values", i, f.OffsetMs, len(f.Values))
		}
	}
	if got.blendShapes[2].Values[17] != 10 {
		t.Fatalf("frame 2 jawOpen = %v, want the second batch's pose", got.blendShapes[2].Values[17])
	}
}

func TestCollectorMergesBatchesOutOfOrder(t *testing.T) {
	c := newCollector()
	c.viseme(0, 0, batchJSON(t, 62, 2, 100)) // frames 62-63
	c.viseme(0, 0, batchJSON(t, 0, 62, 0))   // frames 0-61
	c.viseme(0, 0, batchJSON(t, 63, 2, 500)) // frame 63 again (kept from the first batch) and 64

	got := c.result()

	if len(got.blendShapes) != 65 {
		t.Fatalf("frames = %d, want 65", len(got.blendShapes))
	}
	for i, f := range got.blendShapes {
		if f.OffsetMs != frameToMs(i) {
			t.Fatalf("frame %d offset = %d, want %d", i, f.OffsetMs, frameToMs(i))
		}
	}
	if v := got.blendShapes[63].Values[17]; v != 101 {
		t.Fatalf("duplicate frame 63 = %v, want the first pose 101", v)
	}
	if v := got.blendShapes[64].Values[17]; v != 501 {
		t.Fatalf("frame 64 = %v, want 501", v)
	}
	if last := got.blendShapes[64].OffsetMs; last != 1067 {
		t.Fatalf("frame 64 offset = %d ms", last)
	}
}

func TestCollectorDropsMalformedAnimation(t *testing.T) {
	c := newCollector()
	c.viseme(0, 0, `{"FrameIndex":0,"BlendShapes":[[0.1,0.2]]}`) // wrong width
	c.viseme(0, 0, `{"BlendShapes":[]}`)                         // no frame index
	c.viseme(0, 0, `{"FrameIndex":-3,"BlendShapes":[]}`)         // negative index
	c.viseme(0, 0, `not json`)                                   // unparseable
	c.viseme(0, 0, `{"FrameIndex":5,"BlendShapes":[]}`)          // empty batch, nothing to drop
	c.viseme(0, 0, batchJSON(t, 1, 1, 0))                        // one good frame
	c.viseme(4_375_000, 1, "")                                   // a plain viseme stays a viseme

	got := c.result()

	if len(got.blendShapes) != 1 || got.blendShapes[0].OffsetMs != 17 {
		t.Fatalf("frames = %+v", got.blendShapes)
	}
	if got.droppedFrames != 4 {
		t.Fatalf("dropped = %d, want 4", got.droppedFrames)
	}
	if !reflect.DeepEqual(got.visemes, []tts.Viseme{{OffsetMs: 438, ID: 1}}) {
		t.Fatalf("visemes = %+v", got.visemes)
	}
}

func TestCollectorEmptyAndFinished(t *testing.T) {
	c := newCollector()
	if got := c.result(); got.visemes != nil || got.words != nil || got.blendShapes != nil || got.droppedFrames != 0 {
		t.Fatalf("empty collector = %+v", got)
	}

	c.finished()
	c.finished() // the SDK fires one end event, but a second must not panic
	select {
	case <-c.done:
	default:
		t.Fatal("done not closed after finished")
	}

	before := c.result()
	c.viseme(10_000, 3, "")
	if len(before.visemes) != 0 {
		t.Fatal("result shares state with the collector")
	}
}

func TestTickAndFrameConversion(t *testing.T) {
	for ticks, ms := range map[uint64]int{0: 0, 4_999: 0, 5_000: 1, 10_000: 1, 9_812_500: 981, 19_370_000: 1937, 38_250_000: 3825} {
		if got := ticksToMs(ticks); got != ms {
			t.Errorf("ticksToMs(%d) = %d, want %d", ticks, got, ms)
		}
	}
	for frame, ms := range map[int]int{0: 0, 1: 17, 2: 33, 3: 50, 60: 1000, 273: 4550} {
		if got := frameToMs(frame); got != ms {
			t.Errorf("frameToMs(%d) = %d, want %d", frame, got, ms)
		}
	}
}
