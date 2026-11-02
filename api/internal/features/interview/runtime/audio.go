package runtime

import (
	"bytes"
	"mime"
	"strings"
)

// normalizeAudioType reduces a recorder MIME type ("audio/webm;codecs=opus") to its container type and
// reports whether the engine accepts it.
func normalizeAudioType(value string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return "", false
	}
	switch mediaType = strings.ToLower(mediaType); mediaType {
	case "audio/webm", "audio/ogg", "audio/mp4", "audio/wav":
		return mediaType, true
	case "audio/x-wav", "audio/wave":
		return "audio/wav", true
	}
	return "", false
}

// matchesContainer checks the first bytes of a recording against its declared container, so garbage is
// refused before it reaches the transcriber.
func matchesContainer(mediaType string, data []byte) bool {
	switch mediaType {
	case "audio/webm":
		return bytes.HasPrefix(data, []byte{0x1A, 0x45, 0xDF, 0xA3})
	case "audio/ogg":
		return bytes.HasPrefix(data, []byte("OggS"))
	case "audio/wav":
		return len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE"))
	case "audio/mp4":
		return len(data) >= 8 && bytes.Equal(data[4:8], []byte("ftyp"))
	}
	return false
}

// answerBuffer assembles the chunks of one recorded answer. Chunks arrive in order on one socket; a
// repeated chunk number is a retransmit and is ignored.
type answerBuffer struct {
	id        string
	position  int32
	mediaType string
	next      int
	data      []byte
}

// chunkResult is what adding a chunk did.
type chunkResult int

const (
	chunkStored chunkResult = iota
	chunkRepeated
	chunkComplete
)

// add appends one chunk. It returns an error code when the chunk breaks the rules; the caller then drops
// the buffer.
func (b *answerBuffer) add(chunk AnswerAudio) (chunkResult, string) {
	if chunk.Chunk < b.next {
		return chunkRepeated, ""
	}
	if chunk.Chunk != b.next || chunk.Position != b.position {
		return 0, CodeAudioChunkOrder
	}
	if len(chunk.Audio) > MaxAudioChunkBytes || len(b.data)+len(chunk.Audio) > MaxAnswerAudioBytes || chunk.Chunk >= MaxAnswerChunks {
		return 0, CodeAudioTooLarge
	}
	b.data = append(b.data, chunk.Audio...)
	b.next++
	if !chunk.Final {
		return chunkStored, ""
	}
	if !matchesContainer(b.mediaType, b.data) {
		return 0, CodeAudioFormatUnsupported
	}
	return chunkComplete, ""
}
