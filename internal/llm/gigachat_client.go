package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/logger"
)

const (
	gigaChatBaseURL       = "https://api.giga.chat"
	gigaChatOAuthURL      = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	gigaChatModel         = "GigaChat-3-Ultra"
	gigaChatScope         = "GIGACHAT_API_PERS"
	summarizeSystemPrompt = "Ты — ассистент, который составляет краткую структурированную выжимку по транскрипции встречи на русском языке. " +
		"Выдели ключевые темы, принятые решения и договорённости. Отвечай только текстом выжимки, без вступлений и пояснений."
	answerSystemPrompt = "Ты — ассистент, который отвечает на вопросы по материалам встречи. " +
		"Используй только предоставленные материалы. Если ответа в материалах нет, честно сообщи об этом."
	tokenMaxLifetime  = 28 * time.Minute
	tokenExpiryBuffer = time.Minute
)

type GigaChatClient struct {
	httpClient  *http.Client
	retries     int
	semaphore   chan struct{}
	authKey     string
	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewGigaChatClient(config LLMClientConfig) (*GigaChatClient, error) {
	if config.AuthKey == "" {
		return nil, fmt.Errorf("не задан ключ авторизации GigaChat (LLM_AUTH_KEY)")
	}

	rateLimit := config.RateLimit
	if rateLimit <= 0 {
		rateLimit = 1
	}

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	return &GigaChatClient{
		httpClient: &http.Client{Timeout: config.RequestTimeout, Transport: transport},
		retries:    config.RequestRetries,
		semaphore:  make(chan struct{}, rateLimit),
		authKey:    config.AuthKey,
	}, nil
}

func (c *GigaChatClient) Summarize(ctx context.Context, text string) (string, error) {
	return c.doRequest(ctx, func(ctx context.Context) (string, error) {
		return c.chatCompletion(ctx, []chatMessage{
			newTextMessage("system", summarizeSystemPrompt),
			newTextMessage("user", text),
		})
	})
}

func (c *GigaChatClient) Answer(ctx context.Context, materials string, question string) (string, error) {
	return c.doRequest(ctx, func(ctx context.Context) (string, error) {
		userPrompt := fmt.Sprintf("Материалы встречи:\n%s\n\nВопрос: %s", materials, question)
		return c.chatCompletion(ctx, []chatMessage{
			newTextMessage("system", answerSystemPrompt),
			newTextMessage("user", userPrompt),
		})
	})
}

func (c *GigaChatClient) doRequest(ctx context.Context, request func(ctx context.Context) (string, error)) (string, error) {
	select {
	case c.semaphore <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-c.semaphore }()

	var result string
	var err error
	for r := 0; r <= c.retries; r++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		result, err = request(ctx)
		if err == nil {
			return result, nil
		}
		logger.Log.Warn("Запрос к LLM-сервису завершился ошибкой, повтор", "attempt", r+1, "error", err.Error())
	}
	return "", err
}

type chatMessageContent struct {
	Text string `json:"text,omitempty"`
}

type chatMessage struct {
	Role    string               `json:"role"`
	Content []chatMessageContent `json:"content"`
}

func newTextMessage(role, text string) chatMessage {
	return chatMessage{Role: role, Content: []chatMessageContent{{Text: text}}}
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatCompletionResponse struct {
	Messages []struct {
		Role    string               `json:"role"`
		Content []chatMessageContent `json:"content"`
	} `json:"messages"`
}

type gigaChatErrorResponse struct {
	Message string `json:"message"`
}

func (c *GigaChatClient) chatCompletion(ctx context.Context, messages []chatMessage) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", fmt.Errorf("не удалось получить токен GigaChat: %w", err)
	}

	reqBody, err := json.Marshal(chatCompletionRequest{
		Model:    gigaChatModel,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gigaChatBaseURL+"/v2/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GigaChat API вернул ошибку %d: %s", resp.StatusCode, gigaChatErrorText(body))
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return "", fmt.Errorf("не удалось разобрать ответ GigaChat: %w", err)
	}
	if len(completion.Messages) == 0 {
		return "", fmt.Errorf("GigaChat не вернул ни одного сообщения")
	}

	var text strings.Builder
	for _, part := range completion.Messages[len(completion.Messages)-1].Content {
		text.WriteString(part.Text)
	}
	result := strings.TrimSpace(text.String())
	if result == "" {
		return "", fmt.Errorf("GigaChat вернул пустой ответ")
	}
	return result, nil
}

func gigaChatErrorText(body []byte) string {
	var errResp gigaChatErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		return errResp.Message
	}
	return string(body)
}

func (c *GigaChatClient) getToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	form := url.Values{}
	form.Set("scope", gigaChatScope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gigaChatOAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+c.authKey)
	req.Header.Set("RqUID", uuid.NewString())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("сервис авторизации GigaChat вернул ошибку %d: %s", resp.StatusCode, gigaChatErrorText(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("не удалось разобрать ответ сервиса авторизации: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("сервис авторизации GigaChat не вернул access_token")
	}

	expiry := time.Now().Add(tokenMaxLifetime)
	if tokenResp.ExpiresAt > 0 {
		if parsed := time.UnixMilli(tokenResp.ExpiresAt); parsed.Before(expiry) {
			expiry = parsed
		}
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = expiry.Add(-tokenExpiryBuffer)
	return c.accessToken, nil
}
