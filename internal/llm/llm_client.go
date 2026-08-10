package llm

import (
	"context"
	"net/http"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/logger"
)

type TestLLMClient struct {
	httpClient *http.Client
	retries    int
	semaphore  chan struct{}
}

func NewTestLLMClient(config LLMClientConfig) *TestLLMClient {
	return &TestLLMClient{
		httpClient: &http.Client{Timeout: config.RequestTimeout},
		retries:    config.RequestRetries,
		semaphore:  make(chan struct{}, config.RateLimit),
	}
}

func (c *TestLLMClient) Summarize(ctx context.Context, text string) (string, error) {
	return c.doRequest(func() (string, error) {
		return "Тестовая краткая выжимка по тексту транскрипции.", nil
	})
}

func (c *TestLLMClient) Answer(ctx context.Context, materials string, question string) (string, error) {
	return c.doRequest(func() (string, error) {
		return "Тестовый ответ на вопрос: " + question, nil
	})
}

func (c *TestLLMClient) doRequest(request func() (string, error)) (string, error) {
	c.semaphore <- struct{}{}
	defer func() { <-c.semaphore }()
	var result string
	var err error
	for r := 0; r <= c.retries; r++ {
		result, err = request()
		if err == nil {
			return result, nil
		}
		logger.Log.Warn("Запрос к LLM-сервису завершился ошибкой, повтор", "attempt", r+1, "error", err.Error())
	}
	return "", err
}
