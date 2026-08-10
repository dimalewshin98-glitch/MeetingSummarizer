package speech

import (
	"context"
	"time"
)

type SpeechClientConfig struct {
	RequestTimeout time.Duration
	RequestRetries int
	RateLimit      int
}

type SpeechClientInterface interface {
	Transcribe(ctx context.Context, audioFile []byte) (string, error)
}
