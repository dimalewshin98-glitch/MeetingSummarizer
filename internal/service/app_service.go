package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/bot"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/config"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/llm"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/logger"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/model"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/repository"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/speech"
)

const commandsInfoText = `Я распознаю голосовые встречи и делаю по ним краткую выжимку.

Доступные команды:
start - начало работы
list - список ваших встреч
status <id> - статус обработки встречи
get <id> - транскрипция встречи по id
find <keyword> - поиск встреч по ключевому слову
chat <id> <question> - вопрос по встрече с заданным id
retry <id> - повторная обработка встречи, завершившейся ошибкой
delete <id> - удалить встречу`

const dateTimeLayout = "2006-01-02 15:04:05"

type AppService struct {
	repo         repository.RepositoryInterface
	bot          bot.BotInterface
	config       *config.Config
	speechClient speech.SpeechClientInterface
	llmClient    llm.LLMClientInterface
	jobChan      chan model.Meeting
}

func NewAppService(repo repository.RepositoryInterface, bot bot.BotInterface, speechClient speech.SpeechClientInterface, llmClient llm.LLMClientInterface, config *config.Config) *AppService {
	serviceInstance := &AppService{
		repo:         repo,
		bot:          bot,
		config:       config,
		speechClient: speechClient,
		llmClient:    llmClient,
		jobChan:      make(chan model.Meeting, config.WorkersCount),
	}
	return serviceInstance
}

func (s *AppService) StartWorkers(ctx context.Context) {
	for w := 1; w <= s.config.WorkersCount; w++ {
		go s.worker(ctx, w)
	}
}

func (s *AppService) StartBotListener(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Log.Info("Bot Listener остановлен")
				return
			default:
				msg, err := s.bot.RecieveMessage()
				if err != nil {
					logger.Log.Error("Bot Listener прервал своб работу", "error", err.Error())
					return
				}
				s.jobChan <- model.Meeting{
					UserID:      msg.UserID,
					MessageID:   msg.MessageID,
					AudioFile:   msg.AudioFile,
					TextFile:    msg.TextFile,
					RequestText: msg.Text,
				}
			}
		}
	}()
}

func (s *AppService) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

func (s *AppService) worker(ctx context.Context, workerID int) {
	logger.Log.Info("Воркер запущен", "id", workerID)
	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("Воркер остановлен", "id", workerID)
			return
		default:
			job := <-s.jobChan
			switch job.Status {
			case "":
				s.jobFirstInit(job)
			case model.StatusCreated:
				s.startProcessJob(job)
			case model.StatusProcessing:
				s.jobTranscribe(job)
			case model.StatusTranscribed:
				s.jobSummarize(job)
			case model.StatusSummarized:
				s.jobComplete(job)
			}
		}
	}
}

func (s *AppService) jobFirstInit(job model.Meeting) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.CreateUser(ctx, job.UserID); err != nil {
		logger.Log.Error("Не удалось зарегистрировать пользователя", "userID", job.UserID, "error", err.Error())
		s.reply(job, "Не удалось зарегистрировать пользователя.")
		return
	}
	if len(job.AudioFile) > 0 || len(job.TextFile) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		job.Status = model.StatusCreated
		meetingID, err := s.repo.CreateMeeting(ctx, job)
		if err != nil {
			logger.Log.Error("Не удалось сохранить встречу", "error", err.Error())
			s.reply(job, "Не удалось зарегистрировать встречу.")
			return
		}
		job.MeetingID = meetingID
		logger.Log.Info("Статус встречи изменён", "meetingID", job.MeetingID, "status", job.Status)
		s.replyJob(job)
		s.reply(job, fmt.Sprintf("Запрос зарегистрирован.\nid: %d", job.MeetingID))
		return
	}
	s.handleCommand(job)
}

func (s *AppService) startProcessJob(job model.Meeting) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	job.Status = model.StatusProcessing
	job.StatusUpdatedAt = time.Now()
	if err := s.repo.UpdateMeetingStatus(ctx, job.MeetingID, job.Status, ""); err != nil {
		logger.Log.Error("Не удалось обновить статус встречи", "meetingID", job.MeetingID, "error", err.Error())
		s.jobFail(job, err)
		return
	}
	logger.Log.Info("Статус встречи изменён", "meetingID", job.MeetingID, "status", job.Status)
	s.replyJob(job)
}

func (s *AppService) jobTranscribe(job model.Meeting) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if len(job.AudioFile) > 0 {
		transcriptionText, err := s.speechClient.Transcribe(ctx, job.AudioFile)
		if err != nil {
			s.jobFail(job, err)
			return
		}
		job.TranscriptionText = transcriptionText
	}
	if len(job.TextFile) > 0 {
		job.TranscriptionText = string(job.TextFile)
	}
	job.Status = model.StatusTranscribed
	job.StatusUpdatedAt = time.Now()
	if err := s.repo.UpdateMeeting(ctx, job); err != nil {
		logger.Log.Error("Не удалось сохранить транскрипцию", "meetingID", job.MeetingID, "error", err.Error())
		s.jobFail(job, err)
		return
	}
	logger.Log.Info("Статус встречи изменён", "meetingID", job.MeetingID, "status", job.Status)
	s.replyJob(job)
}

func (s *AppService) jobSummarize(job model.Meeting) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	summaryText, err := s.llmClient.Summarize(ctx, job.TranscriptionText)
	if err != nil {
		s.jobFail(job, err)
		return
	}
	job.SummaryText = summaryText
	job.Status = model.StatusSummarized
	job.StatusUpdatedAt = time.Now()
	if err := s.repo.UpdateMeeting(ctx, job); err != nil {
		logger.Log.Error("Не удалось сохранить выжимку", "meetingID", job.MeetingID, "error", err.Error())
		s.jobFail(job, err)
		return
	}
	logger.Log.Info("Статус встречи изменён", "meetingID", job.MeetingID, "status", job.Status)
	s.replyJob(job)
}

func (s *AppService) jobComplete(job model.Meeting) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	job.Status = model.StatusCompleted
	job.StatusUpdatedAt = time.Now()
	if err := s.repo.UpdateMeetingStatus(ctx, job.MeetingID, job.Status, ""); err != nil {
		logger.Log.Error("Не удалось обновить статус встречи", "meetingID", job.MeetingID, "error", err.Error())
		s.jobFail(job, err)
		return
	}
	logger.Log.Info("Статус встречи изменён", "meetingID", job.MeetingID, "status", job.Status)
	s.reply(job, fmt.Sprintf("Готово.\nid: %d\nВыжимка: %s", job.MeetingID, job.SummaryText))
}

func (s *AppService) jobFail(job model.Meeting, err error) {
	logger.Log.Error("Обработка встречи завершилась ошибкой", "meetingID", job.MeetingID, "error", err.Error())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	job.Status = model.StatusFailed
	job.ErrorText = err.Error()
	job.StatusUpdatedAt = time.Now()
	if updErr := s.repo.UpdateMeetingStatus(ctx, job.MeetingID, job.Status, job.ErrorText); updErr != nil {
		logger.Log.Error("Не удалось сохранить статус ошибки", "meetingID", job.MeetingID, "error", updErr.Error())
	} else {
		logger.Log.Info("Статус встречи изменён", "meetingID", job.MeetingID, "status", job.Status)
	}
	s.reply(job, fmt.Sprintf("Обработка встречи #%d завершилась ошибкой.", job.MeetingID))
}

func (s *AppService) reply(job model.Meeting, text string) {
	resMessage := bot.BotResponseMessage{MessageID: job.MessageID, UserID: job.UserID, Text: text, Date: int(time.Now().Unix())}
	if err := s.bot.SendMessage(resMessage); err != nil {
		logger.Log.Error("Не удалось отправить ответ пользователю", "userID", job.UserID, "error", err.Error())
	}
}

func (s *AppService) replyJob(job model.Meeting) {
	select {
	case s.jobChan <- job:
	default:
		err := errors.New("Высокая нагрузка на сервис")
		logger.Log.Error("Не удалось обработать встречу", "meetingID", job.MeetingID, "error", err.Error())
		s.jobFail(job, err)
	}
}

func (s *AppService) handleCommand(job model.Meeting) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fields := strings.Fields(job.RequestText)
	if len(fields) == 0 {
		s.reply(job, commandsInfoText)
		return
	} else {
		command := strings.ToLower(fields[0])
		args := fields[1:]
		switch command {
		case "start":
			s.reply(job, commandsInfoText)
		case "list":
			s.cmdList(ctx, job, args)
		case "status":
			s.cmdStatus(ctx, job, args)
		case "get":
			s.cmdGet(ctx, job, args)
		case "find":
			s.cmdFind(ctx, job, args)
		case "chat":
			s.cmdChat(ctx, job, args)
		case "retry":
			s.cmdRetry(ctx, job, args)
		case "delete":
			s.cmdDelete(ctx, job, args)
		default:
			s.reply(job, "Неизвестная команда.\n\n"+commandsInfoText)
		}
	}
}

func (s *AppService) cmdList(ctx context.Context, job model.Meeting, args []string) {
	if len(args) >= 1 {
		s.reply(job, "Использование: list")
		return
	}
	meetings, err := s.repo.ListMeetings(ctx, job.UserID)
	if err != nil {
		s.reply(job, "Не удалось получить список встреч.")
		return
	}
	if len(meetings) == 0 {
		s.reply(job, "У вас пока нет встреч.")
		return
	}
	resultText := "Список встреч:\n"
	for _, m := range meetings {
		if m.SummaryText == "" {
			if m.Status == model.StatusFailed {
				resultText += fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nОшибка: %s\n", m.MeetingID, m.CreatedAt.Format(dateTimeLayout), m.Status, m.ErrorText)
			} else {
				resultText += fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nРезультат: В обработке...\n", m.MeetingID, m.CreatedAt.Format(dateTimeLayout), m.Status)
			}
		} else {
			resultText += fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nРезультат: %s\n", m.MeetingID, m.CreatedAt.Format(dateTimeLayout), m.Status, m.SummaryText)
		}
	}
	s.reply(job, resultText)
}

func (s *AppService) cmdStatus(ctx context.Context, job model.Meeting, args []string) {
	if len(args) != 1 {
		s.reply(job, "Использование: status <id>")
		return
	}
	meetingID, err := strconv.Atoi(args[0])
	if err != nil {
		s.reply(job, "Использование: status <id>")
		return
	}
	meeting, err := s.repo.GetMeeting(ctx, job.UserID, meetingID)
	if err != nil {
		s.reply(job, "Встреча не найдена.")
		return
	}
	resultText := "Статус встречи:\n"
	resultText += fmt.Sprintf("№ %d\nСтатус: %s\nДата создания: %s\nДата обновления статуса: %s",
		meeting.MeetingID, meeting.Status, meeting.CreatedAt.Format(dateTimeLayout), meeting.StatusUpdatedAt.Format(dateTimeLayout))
	if meeting.Status == model.StatusFailed {
		resultText += fmt.Sprintf("\nОшибка: %s", meeting.ErrorText)
	}
	s.reply(job, resultText)
}

func (s *AppService) cmdGet(ctx context.Context, job model.Meeting, args []string) {
	if len(args) != 1 {
		s.reply(job, "Использование: get <id>")
		return
	}
	meetingID, err := strconv.Atoi(args[0])
	if err != nil {
		s.reply(job, "Использование: get <id>")
		return
	}
	meeting, err := s.repo.GetMeeting(ctx, job.UserID, meetingID)
	if err != nil {
		s.reply(job, "Встреча не найдена.")
		return
	}
	resultText := "Найденная встреча:\n"
	if meeting.SummaryText == "" {
		if meeting.Status == model.StatusFailed {
			resultText = fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nОшибка: %s\n", meeting.MeetingID, meeting.CreatedAt.Format(dateTimeLayout), meeting.Status, meeting.ErrorText)
		} else {
			resultText = fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nРезультат: В обработке...\n", meeting.MeetingID, meeting.CreatedAt.Format(dateTimeLayout), meeting.Status)
		}
	} else {
		resultText = fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nРезультат: %s\n", meeting.MeetingID, meeting.CreatedAt.Format(dateTimeLayout), meeting.Status, meeting.SummaryText)
	}
	s.reply(job, resultText)
}

func (s *AppService) cmdFind(ctx context.Context, job model.Meeting, args []string) {
	if len(args) != 1 {
		s.reply(job, "Использование: find <keyword>")
		return
	}
	if args[0] == "" {
		s.reply(job, "Использование: find <keyword>")
		return
	}
	meetings, err := s.repo.FindMeetings(ctx, job.UserID, args[0])
	if err != nil {
		s.reply(job, "Не удалось выполнить поиск.")
		return
	}
	if len(meetings) == 0 {
		s.reply(job, "Ничего не найдено.")
		return
	}
	resultText := "Найденные встречи:\n"
	for _, m := range meetings {
		if m.SummaryText == "" {
			resultText += fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nРезультат: В обработке...\n", m.MeetingID, m.CreatedAt.Format(dateTimeLayout), m.Status)
		} else {
			resultText += fmt.Sprintf("№ %d\nДата создания: %s\nСтатус: %s\nРезультат: %s\n", m.MeetingID, m.CreatedAt.Format(dateTimeLayout), m.Status, m.SummaryText)
		}
	}
	s.reply(job, resultText)
}

func (s *AppService) cmdChat(ctx context.Context, job model.Meeting, args []string) {
	if len(args) < 2 {
		s.reply(job, "Использование: chat <id> <question>")
		return
	}
	meetingID, err := strconv.Atoi(args[0])
	if err != nil {
		s.reply(job, "Использование: chat <id> <question>")
		return
	}
	question := strings.Join(args[1:], " ")
	if question == "" {
		s.reply(job, "Использование: chat <id> <question>")
		return
	}
	meeting, err := s.repo.GetMeeting(ctx, job.UserID, meetingID)
	if err != nil {
		s.reply(job, "Встреча не найдена.")
		return
	}
	if meeting.Status != model.StatusCompleted {
		s.reply(job, "Встреча еще не обработана.")
		return
	}
	answer, err := s.llmClient.Answer(ctx, meeting.SummaryText, question)
	if err != nil {
		s.reply(job, "Не удалось получить ответ.")
		return
	}
	s.reply(job, answer)
}

func (s *AppService) cmdRetry(ctx context.Context, job model.Meeting, args []string) {
	if len(args) != 1 {
		s.reply(job, "Использование: retry <id>")
		return
	}
	meetingID, err := strconv.Atoi(args[0])
	if err != nil {
		s.reply(job, "Использование: retry <id>")
		return
	}
	meeting, err := s.repo.GetMeeting(ctx, job.UserID, meetingID)
	if err != nil {
		s.reply(job, "Встреча не найдена.")
		return
	}
	if meeting.Status != model.StatusFailed {
		s.reply(job, "Повторно обрабатывать можно только встречи со статусом failed.")
		return
	}
	meeting.Status = model.StatusCreated
	meeting.ErrorText = ""
	meeting.StatusUpdatedAt = time.Now()
	if err := s.repo.UpdateMeetingStatus(ctx, meeting.MeetingID, meeting.Status, meeting.ErrorText); err != nil {
		s.reply(job, "Не удалось перезапустить обработку.")
		return
	}
	logger.Log.Info("Статус встречи изменён", "meetingID", meeting.MeetingID, "status", meeting.Status)
	s.jobChan <- meeting
	resultText := fmt.Sprintf("Встреча № %d поставлена на повторную обработку.", meeting.MeetingID)
	s.reply(job, resultText)
}

func (s *AppService) cmdDelete(ctx context.Context, job model.Meeting, args []string) {
	if len(args) != 1 {
		s.reply(job, "Использование: delete <id>")
		return
	}
	meetingID, err := strconv.Atoi(args[0])
	if err != nil {
		s.reply(job, "Использование: delete <id>")
		return
	}
	if err := s.repo.DeleteMeeting(ctx, job.UserID, meetingID); err != nil {
		s.reply(job, "Не удалось удалить встречу.")
		return
	}
	resultText := fmt.Sprintf("Встреча № %d удалена.", meetingID)
	s.reply(job, resultText)
}
