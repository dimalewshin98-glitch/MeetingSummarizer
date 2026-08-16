package speech

import "fmt"

const ProviderMock = "mock"

func NewSpeechClient(provider string, config SpeechClientConfig) (SpeechClientInterface, error) {
	switch provider {
	case ProviderMock, "":
		return NewTestSpeechClient(config), nil
	default:
		return nil, fmt.Errorf("неизвестный speech-провайдер: %s", provider)
	}
}
