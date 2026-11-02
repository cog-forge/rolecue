package sdk

import (
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

// SDK audio offsets are in 100-nanosecond ticks.
const ticksPerMs = 10_000

// collector gathers one synthesis's events. They do not arrive strictly in audio order, so the timelines
// are sorted once the synthesis has finished.
type collector struct {
	mu         sync.Mutex
	visemes    []tts.Viseme
	words      []tts.WordBoundary
	animations []string
	done       chan struct{}
	finish     sync.Once
}

func newCollector() *collector { return &collector{done: make(chan struct{})} }

func (c *collector) viseme(offsetTicks uint64, id uint32, animation string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if animation != "" {
		c.animations = append(c.animations, animation)
		return
	}
	c.visemes = append(c.visemes, tts.Viseme{OffsetMs: ticksToMs(offsetTicks), ID: int(id)})
}

func (c *collector) word(offsetTicks uint64, duration time.Duration, text string, kind boundaryKind) {
	text = strings.TrimSpace(text)
	if kind != boundaryWord || text == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.words = append(c.words, tts.WordBoundary{
		Text:       text,
		OffsetMs:   ticksToMs(offsetTicks),
		DurationMs: int(duration.Round(time.Millisecond) / time.Millisecond),
	})
}

func (c *collector) finished() { c.finish.Do(func() { close(c.done) }) }

// droppedFrames counts blend-shape frames left out as malformed.
type timelines struct {
	visemes       []tts.Viseme
	blendShapes   []tts.BlendShapeFrame
	words         []tts.WordBoundary
	droppedFrames int
}

// result copies, so late events cannot change it.
func (c *collector) result() timelines {
	c.mu.Lock()
	visemes := slices.Clone(c.visemes)
	words := slices.Clone(c.words)
	animations := slices.Clone(c.animations)
	c.mu.Unlock()

	slices.SortStableFunc(visemes, func(a, b tts.Viseme) int { return a.OffsetMs - b.OffsetMs })
	slices.SortStableFunc(words, func(a, b tts.WordBoundary) int { return a.OffsetMs - b.OffsetMs })
	frames, dropped := mergeAnimations(animations)
	return timelines{visemes: visemes, blendShapes: frames, words: words, droppedFrames: dropped}
}

// animationBatch holds consecutive 60 FPS frames starting at FrameIndex.
type animationBatch struct {
	FrameIndex  *int        `json:"FrameIndex"`
	BlendShapes [][]float32 `json:"BlendShapes"`
}

// mergeAnimations orders the batches by frame index; a frame index seen twice keeps its first pose.
func mergeAnimations(batches []string) ([]tts.BlendShapeFrame, int) {
	poses := map[int][]float32{}
	dropped := 0
	for _, raw := range batches {
		var b animationBatch
		if err := json.Unmarshal([]byte(raw), &b); err != nil || b.FrameIndex == nil || *b.FrameIndex < 0 {
			dropped += max(len(b.BlendShapes), 1)
			continue
		}
		for i, values := range b.BlendShapes {
			if len(values) != tts.BlendShapeCount {
				dropped++
				continue
			}
			if _, seen := poses[*b.FrameIndex+i]; !seen {
				poses[*b.FrameIndex+i] = values
			}
		}
	}
	if len(poses) == 0 {
		return nil, dropped
	}
	indexes := make([]int, 0, len(poses))
	for i := range poses {
		indexes = append(indexes, i)
	}
	slices.Sort(indexes)
	frames := make([]tts.BlendShapeFrame, len(indexes))
	for n, i := range indexes {
		frames[n] = tts.BlendShapeFrame{OffsetMs: frameToMs(i), Values: poses[i]}
	}
	return frames, dropped
}

func ticksToMs(ticks uint64) int { return int((ticks + ticksPerMs/2) / ticksPerMs) }

func frameToMs(index int) int { return (index*1000 + tts.BlendShapeFPS/2) / tts.BlendShapeFPS }
