package main

import (
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/bot"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/config"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/handler"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/llm"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/mocks"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/service"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/speech"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type stubBot struct{}

func (stubBot) Start() {}

func (stubBot) SendMessage(bot.BotResponseMessage) error { return nil }

func (stubBot) RecieveMessage() (bot.BotRequestMessage, error) {
	return bot.BotRequestMessage{}, errors.New("no incoming messages")
}

func TestPingHandler(t *testing.T) {
	type want struct {
		statusCode int
		response   string
		dbResponse error
	}
	tests := []struct {
		name        string
		request     string
		requestType string
		want        want
	}{
		{
			name: "test 1 | Success",
			want: want{
				statusCode: 200,
				response:   "",
				dbResponse: nil,
			},
			request:     "/ping",
			requestType: "GET",
		},
		{
			name: "test 2 | Unsuccess | Request type error",
			want: want{
				statusCode: 405,
				response:   "Method not allowed\n",
				dbResponse: nil,
			},
			request:     "/ping",
			requestType: "POST",
		},
		{
			name: "test 3 | Unsuccess | Repo ping error",
			want: want{
				statusCode: 500,
				response:   "db connection failed\n",
				dbResponse: errors.New("db connection failed"),
			},
			request:     "/ping",
			requestType: "GET",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepository := mocks.NewMockRepositoryInterface(ctrl)
			if tt.requestType == "GET" {
				mockedRepository.EXPECT().
					Ping(gomock.Any()).
					Return(tt.want.dbResponse)
			}
			mockedConfig := &config.Config{
				ServerHostPort: "localhost:8080",
				WorkersCount:   1,
			}
			appService := service.NewAppService(mockedRepository, stubBot{}, speech.NewTestSpeechClient(speech.SpeechClientConfig{RateLimit: 1}), llm.NewTestLLMClient(llm.LLMClientConfig{RateLimit: 1}), mockedConfig)
			requestsHandler := handler.NewRequestsHandler(appService)
			request := httptest.NewRequest(tt.requestType, tt.request, nil)
			w := httptest.NewRecorder()
			requestsHandler.Ping(w, request)
			resBytes, _ := io.ReadAll(w.Body)
			assert.Equal(t, tt.want.statusCode, w.Result().StatusCode)
			assert.Equal(t, tt.want.response, string(resBytes))
		})
	}
}
