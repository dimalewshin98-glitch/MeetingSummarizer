package speech

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/logger"
)

type TestSpeechClient struct {
	httpClient *http.Client
	retries    int
	semaphore  chan struct{}
}

func NewTestSpeechClient(config SpeechClientConfig) *TestSpeechClient {
	return &TestSpeechClient{
		httpClient: &http.Client{Timeout: config.RequestTimeout},
		retries:    config.RequestRetries,
		semaphore:  make(chan struct{}, config.RateLimit),
	}
}

func (c *TestSpeechClient) Transcribe(ctx context.Context, audioFile []byte) (string, error) {
	return c.doRequest(func() (string, error) {
		return fmt.Sprintf("Тестовая транскрипция аудиофайла размером %d байт.", len(audioFile)), nil
	})
}

func (c *TestSpeechClient) doRequest(request func() (string, error)) (string, error) {
	c.semaphore <- struct{}{}
	defer func() { <-c.semaphore }()
	var result string
	var err error
	for r := 0; r <= c.retries; r++ {
		result, err = request()
		if err == nil {
			return result, nil
		}
		logger.Log.Warn("Запрос к севису-транскрипция завершился ошибкой, повтор", "attempt", r+1, "error", err.Error())
	}
	return "", err
}
