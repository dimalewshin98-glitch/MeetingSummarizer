package llm

import (
	"context"
	"time"
)

type LLMClientConfig struct {
	RequestTimeout time.Duration
	RequestRetries int
	RateLimit      int
}

type LLMClientInterface interface {
	Summarize(ctx context.Context, text string) (string, error)
	Answer(ctx context.Context, materials string, question string) (string, error)
}
