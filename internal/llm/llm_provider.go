package llm

import "fmt"

const ProviderMock = "mock"

func NewLLMClient(provider string, config LLMClientConfig) (LLMClientInterface, error) {
	switch provider {
	case ProviderMock, "":
		return NewTestLLMClient(config), nil
	default:
		return nil, fmt.Errorf("неизвестный LLM-провайдер: %s", provider)
	}
}
