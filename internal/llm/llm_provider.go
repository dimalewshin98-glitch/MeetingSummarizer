package llm

import "fmt"

const (
	ProviderMock     = "mock"
	ProviderGigaChat = "gigachat"
)

func NewLLMClient(provider string, config LLMClientConfig) (LLMClientInterface, error) {
	switch provider {
	case ProviderMock, "":
		return NewTestLLMClient(config), nil
	case ProviderGigaChat:
		return NewGigaChatClient(config)
	default:
		return nil, fmt.Errorf("неизвестный LLM-провайдер: %s", provider)
	}
}
