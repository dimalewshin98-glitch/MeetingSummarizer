package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/bot"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/llm"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/logger"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/mocks"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/model"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/repository"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/speech"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	_ = logger.Initialize("error")
	os.Exit(m.Run())
}

type fakeBot struct {
	sent []bot.BotResponseMessage
}

func (f *fakeBot) Start() {}

func (f *fakeBot) SendMessage(message bot.BotResponseMessage) error {
	f.sent = append(f.sent, message)
	return nil
}

func (f *fakeBot) RecieveMessage() (bot.BotRequestMessage, error) {
	return bot.BotRequestMessage{}, errors.New("no incoming messages")
}

func newTestAppService(repo repository.RepositoryInterface) (*AppService, *fakeBot) {
	fb := &fakeBot{}
	return &AppService{
		repo:         repo,
		bot:          fb,
		speechClient: speech.NewTestSpeechClient(speech.SpeechClientConfig{RateLimit: 1}),
		llmClient:    llm.NewTestLLMClient(llm.LLMClientConfig{RateLimit: 1}),
		jobChan:      make(chan model.Meeting, 10),
	}, fb
}

var testTime = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

func TestCmdStatus(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		setupMock func(m *mocks.MockRepositoryInterface)
		wantReply string
	}{
		{
			name: "test 1 | Success | Completed meeting",
			arg:  "1",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 1).Return(model.Meeting{
					MeetingID: 1, Status: model.StatusCompleted, CreatedAt: testTime, StatusUpdatedAt: testTime,
				}, nil)
			},
			wantReply: "Статус встречи:\n№ 1\nСтатус: completed\nДата создания: " + testTime.Format(dateTimeLayout) + "\nДата обновления статуса: " + testTime.Format(dateTimeLayout),
		},
		{
			name: "test 2 | Success | Failed meeting includes error text",
			arg:  "2",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 2).Return(model.Meeting{
					MeetingID: 2, Status: model.StatusFailed, ErrorText: "boom", CreatedAt: testTime, StatusUpdatedAt: testTime,
				}, nil)
			},
			wantReply: "Статус встречи:\n№ 2\nСтатус: failed\nДата создания: " + testTime.Format(dateTimeLayout) + "\nДата обновления статуса: " + testTime.Format(dateTimeLayout) + "\nОшибка: boom",
		},
		{
			name:      "test 3 | Unsuccess | Non-numeric id",
			arg:       "abc",
			setupMock: func(m *mocks.MockRepositoryInterface) {},
			wantReply: "Использование: status <id>",
		},
		{
			name: "test 4 | Unsuccess | Meeting not found",
			arg:  "3",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 3).Return(model.Meeting{}, repository.ErrMeetingNotFound)
			},
			wantReply: "Встреча не найдена.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.cmdStatus(context.Background(), model.Meeting{UserID: 42}, tt.arg)
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}

func TestCmdGet(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		setupMock func(m *mocks.MockRepositoryInterface)
		wantReply string
	}{
		{
			name: "test 1 | Success | Meeting with summary",
			arg:  "1",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 1).Return(model.Meeting{
					MeetingID: 1, Status: model.StatusCompleted, SummaryText: "summary", CreatedAt: testTime,
				}, nil)
			},
			wantReply: "№ 1\nДата создания: " + testTime.Format(dateTimeLayout) + "\nСтатус: completed\nРезультат: summary\n",
		},
		{
			name: "test 2 | Success | Meeting still processing",
			arg:  "2",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 2).Return(model.Meeting{
					MeetingID: 2, Status: model.StatusTranscribed, CreatedAt: testTime,
				}, nil)
			},
			wantReply: "№ 2\nДата создания: " + testTime.Format(dateTimeLayout) + "\nСтатус: transcribed\nРезультат: В обработке...\n",
		},
		{
			name:      "test 3 | Unsuccess | Non-numeric id",
			arg:       "xyz",
			setupMock: func(m *mocks.MockRepositoryInterface) {},
			wantReply: "Использование: get <id>",
		},
		{
			name: "test 4 | Unsuccess | Meeting not found",
			arg:  "3",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 3).Return(model.Meeting{}, repository.ErrMeetingNotFound)
			},
			wantReply: "Встреча не найдена.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.cmdGet(context.Background(), model.Meeting{UserID: 42}, tt.arg)
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}

func TestCmdList(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m *mocks.MockRepositoryInterface)
		wantReply string
	}{
		{
			name: "test 1 | Success | No meetings",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().ListMeetings(gomock.Any(), 42).Return(nil, nil)
			},
			wantReply: "У вас пока нет встреч.",
		},
		{
			name: "test 2 | Success | One meeting",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().ListMeetings(gomock.Any(), 42).Return([]model.Meeting{
					{MeetingID: 1, Status: model.StatusCompleted, SummaryText: "summary", CreatedAt: testTime},
				}, nil)
			},
			wantReply: "Список встреч:\n№ 1\nДата создания: " + testTime.Format(dateTimeLayout) + "\nСтатус: completed\nРезультат: summary\n",
		},
		{
			name: "test 3 | Unsuccess | Repository error",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().ListMeetings(gomock.Any(), 42).Return(nil, errors.New("db error"))
			},
			wantReply: "Не удалось получить список встреч.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.cmdList(context.Background(), model.Meeting{UserID: 42})
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}

func TestCmdFind(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		setupMock func(m *mocks.MockRepositoryInterface)
		wantReply string
	}{
		{
			name:      "test 1 | Unsuccess | Empty keyword",
			arg:       "",
			setupMock: func(m *mocks.MockRepositoryInterface) {},
			wantReply: "Использование: find <keyword>",
		},
		{
			name: "test 2 | Success | Nothing found",
			arg:  "keyword",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().FindMeetings(gomock.Any(), 42, "keyword").Return(nil, nil)
			},
			wantReply: "Ничего не найдено.",
		},
		{
			name: "test 3 | Success | Meeting found",
			arg:  "keyword",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().FindMeetings(gomock.Any(), 42, "keyword").Return([]model.Meeting{
					{MeetingID: 5, Status: model.StatusCompleted, SummaryText: "summary", CreatedAt: testTime},
				}, nil)
			},
			wantReply: "Найденные встречи:\n№ 5\nДата создания: " + testTime.Format(dateTimeLayout) + "\nСтатус: completed\nРезультат: summary\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.cmdFind(context.Background(), model.Meeting{UserID: 42}, tt.arg)
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}

func TestCmdChat(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		arg2      string
		setupMock func(m *mocks.MockRepositoryInterface)
		wantReply string
	}{
		{
			name:      "test 1 | Unsuccess | Missing question",
			arg:       "1",
			arg2:      "",
			setupMock: func(m *mocks.MockRepositoryInterface) {},
			wantReply: "Использование: chat <id> <текст вопроса>",
		},
		{
			name: "test 2 | Unsuccess | Meeting not found",
			arg:  "1",
			arg2: "question",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 1).Return(model.Meeting{}, repository.ErrMeetingNotFound)
			},
			wantReply: "Встреча не найдена.",
		},
		{
			name: "test 3 | Unsuccess | Meeting not completed yet",
			arg:  "1",
			arg2: "question",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 1).Return(model.Meeting{MeetingID: 1, Status: model.StatusProcessing}, nil)
			},
			wantReply: "Встреча еще не обработана.",
		},
		{
			name: "test 4 | Success | Answer returned",
			arg:  "1",
			arg2: "question",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().GetMeeting(gomock.Any(), 42, 1).Return(model.Meeting{MeetingID: 1, Status: model.StatusCompleted, SummaryText: "summary"}, nil)
			},
			wantReply: "Тестовый ответ на вопрос: question",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.cmdChat(context.Background(), model.Meeting{UserID: 42}, tt.arg, tt.arg2)
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}

func TestCmdDelete(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		setupMock func(m *mocks.MockRepositoryInterface)
		wantReply string
	}{
		{
			name:      "test 1 | Unsuccess | Non-numeric id",
			arg:       "abc",
			setupMock: func(m *mocks.MockRepositoryInterface) {},
			wantReply: "Использование: delete <id>",
		},
		{
			name: "test 2 | Success | Meeting deleted",
			arg:  "1",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().DeleteMeeting(gomock.Any(), 42, 1).Return(nil)
			},
			wantReply: "Встреча № 1 удалена.",
		},
		{
			name: "test 3 | Unsuccess | Repository error",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().DeleteMeeting(gomock.Any(), 42, 2).Return(errors.New("db error"))
			},
			arg:       "2",
			wantReply: "Не удалось удалить встречу.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.cmdDelete(context.Background(), model.Meeting{UserID: 42}, tt.arg)
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}

func TestHandleCommand(t *testing.T) {
	tests := []struct {
		name        string
		requestText string
		wantReply   string
	}{
		{
			name:        "test 1 | Success | Empty message shows info",
			requestText: "",
			wantReply:   commandsInfoText,
		},
		{
			name:        "test 2 | Success | Start command shows info",
			requestText: "start",
			wantReply:   commandsInfoText,
		},
		{
			name:        "test 3 | Unsuccess | Unknown command",
			requestText: "foobar",
			wantReply:   "Неизвестная команда.\n\n" + commandsInfoText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			s, fb := newTestAppService(mockedRepo)
			s.handleCommand(model.Meeting{UserID: 42, RequestText: tt.requestText})
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}

func TestJobFirstInit(t *testing.T) {
	t.Run("test 1 | Unsuccess | CreateUser fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
		mockedRepo.EXPECT().CreateUser(gomock.Any(), 42).Return(errors.New("db error"))
		s, fb := newTestAppService(mockedRepo)
		s.jobFirstInit(model.Meeting{UserID: 42, AudioFile: []byte("audio")})
		assert.Len(t, fb.sent, 1)
		assert.Equal(t, "Не удалось зарегистрировать пользователя.", fb.sent[0].Text)
	})

	t.Run("test 2 | Success | Audio file registers a new meeting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
		mockedRepo.EXPECT().CreateUser(gomock.Any(), 42).Return(nil)
		mockedRepo.EXPECT().CreateMeeting(gomock.Any(), gomock.Any()).Return(7, nil)
		s, fb := newTestAppService(mockedRepo)
		s.jobFirstInit(model.Meeting{UserID: 42, AudioFile: []byte("audio")})
		assert.Len(t, fb.sent, 1)
		assert.Equal(t, "Запрос зарегистрирован.\nid: 7", fb.sent[0].Text)
		select {
		case queued := <-s.jobChan:
			assert.Equal(t, model.StatusCreated, queued.Status)
			assert.Equal(t, 7, queued.MeetingID)
		default:
			t.Fatal("expected meeting to be enqueued for further processing")
		}
	})

	t.Run("test 3 | Unsuccess | CreateMeeting fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
		mockedRepo.EXPECT().CreateUser(gomock.Any(), 42).Return(nil)
		mockedRepo.EXPECT().CreateMeeting(gomock.Any(), gomock.Any()).Return(0, errors.New("db error"))
		s, fb := newTestAppService(mockedRepo)
		s.jobFirstInit(model.Meeting{UserID: 42, AudioFile: []byte("audio")})
		assert.Len(t, fb.sent, 1)
		assert.Equal(t, "Не удалось зарегистрировать встречу.", fb.sent[0].Text)
	})

	t.Run("test 4 | Success | No file falls back to command handling", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
		mockedRepo.EXPECT().CreateUser(gomock.Any(), 42).Return(nil)
		s, fb := newTestAppService(mockedRepo)
		s.jobFirstInit(model.Meeting{UserID: 42, RequestText: "start"})
		assert.Len(t, fb.sent, 1)
		assert.Equal(t, commandsInfoText, fb.sent[0].Text)
	})
}

func TestStartProcessJob(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(m *mocks.MockRepositoryInterface)
		wantReply    string
		wantEnqueued model.ProcessingStatus
	}{
		{
			name: "test 1 | Success | Meeting moves to processing",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusProcessing, "").Return(nil)
			},
			wantEnqueued: model.StatusProcessing,
		},
		{
			name: "test 2 | Unsuccess | Repository error fails the job",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusProcessing, "").Return(errors.New("db error"))
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusFailed, "db error").Return(nil)
			},
			wantReply: "Обработка встречи #1 завершилась ошибкой.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.startProcessJob(model.Meeting{MeetingID: 1})
			if tt.wantEnqueued != "" {
				assert.Len(t, fb.sent, 0)
				queued := <-s.jobChan
				assert.Equal(t, tt.wantEnqueued, queued.Status)
			} else {
				assert.Len(t, fb.sent, 1)
				assert.Equal(t, tt.wantReply, fb.sent[0].Text)
			}
		})
	}
}

func TestJobTranscribe(t *testing.T) {
	tests := []struct {
		name             string
		setupMock        func(m *mocks.MockRepositoryInterface)
		wantReply        string
		wantEnqueued     model.ProcessingStatus
		wantEnqueuedText string
	}{
		{
			name: "test 1 | Success | Text file is transcribed as-is",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeeting(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantEnqueued:     model.StatusTranscribed,
			wantEnqueuedText: "meeting text",
		},
		{
			name: "test 2 | Unsuccess | Repository error fails the job",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeeting(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusFailed, "db error").Return(nil)
			},
			wantReply: "Обработка встречи #1 завершилась ошибкой.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.jobTranscribe(model.Meeting{MeetingID: 1, TextFile: []byte("meeting text")})
			if tt.wantEnqueued != "" {
				assert.Len(t, fb.sent, 0)
				queued := <-s.jobChan
				assert.Equal(t, tt.wantEnqueued, queued.Status)
				assert.Equal(t, tt.wantEnqueuedText, queued.TranscriptionText)
			} else {
				assert.Len(t, fb.sent, 1)
				assert.Equal(t, tt.wantReply, fb.sent[0].Text)
			}
		})
	}
}

func TestJobSummarize(t *testing.T) {
	tests := []struct {
		name                string
		setupMock           func(m *mocks.MockRepositoryInterface)
		wantReply           string
		wantEnqueued        model.ProcessingStatus
		wantEnqueuedSummary string
	}{
		{
			name: "test 1 | Success | Summary is saved",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeeting(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantEnqueued:        model.StatusSummarized,
			wantEnqueuedSummary: "Тестовая краткая выжимка по тексту транскрипции.",
		},
		{
			name: "test 2 | Unsuccess | Repository error fails the job",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeeting(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusFailed, "db error").Return(nil)
			},
			wantReply: "Обработка встречи #1 завершилась ошибкой.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.jobSummarize(model.Meeting{MeetingID: 1, TranscriptionText: "meeting text"})
			if tt.wantEnqueued != "" {
				assert.Len(t, fb.sent, 0)
				queued := <-s.jobChan
				assert.Equal(t, tt.wantEnqueued, queued.Status)
				assert.Equal(t, tt.wantEnqueuedSummary, queued.SummaryText)
			} else {
				assert.Len(t, fb.sent, 1)
				assert.Equal(t, tt.wantReply, fb.sent[0].Text)
			}
		})
	}
}

func TestJobComplete(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m *mocks.MockRepositoryInterface)
		wantReply string
	}{
		{
			name: "test 1 | Success | Meeting completed with summary",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusCompleted, "").Return(nil)
			},
			wantReply: "Готово.\nid: 1\nВыжимка: summary",
		},
		{
			name: "test 2 | Unsuccess | Repository error fails the job",
			setupMock: func(m *mocks.MockRepositoryInterface) {
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusCompleted, "").Return(errors.New("db error"))
				m.EXPECT().UpdateMeetingStatus(gomock.Any(), 1, model.StatusFailed, "db error").Return(nil)
			},
			wantReply: "Обработка встречи #1 завершилась ошибкой.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockedRepo := mocks.NewMockRepositoryInterface(ctrl)
			tt.setupMock(mockedRepo)
			s, fb := newTestAppService(mockedRepo)
			s.jobComplete(model.Meeting{MeetingID: 1, SummaryText: "summary"})
			assert.Len(t, fb.sent, 1)
			assert.Equal(t, tt.wantReply, fb.sent[0].Text)
		})
	}
}
