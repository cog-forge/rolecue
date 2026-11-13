// Package stt defines the provider-neutral boundary for complete answer recordings.
package stt

import (
	"context"
	"errors"
	"time"
)

type Audio struct {
	Data     []byte
	MimeType string
}

type Transcript struct {
	Text     string
	Duration time.Duration
}

type Transcriber interface {
	Transcribe(context.Context, Audio) (Transcript, error)
}

var (
	ErrNoSpeech         = errors.New("stt: no speech")
	ErrUnsupportedAudio = errors.New("stt: unsupported audio")
	ErrUnavailable      = errors.New("stt: provider unavailable")
)
