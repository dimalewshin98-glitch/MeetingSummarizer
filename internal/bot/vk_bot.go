package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/logger"
)

var ErrSessionExpired = errors.New("long poll session expired")
var ErrTooManyAttachments = errors.New("к сообщению можно приложить только один файл")

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d: %s", e.Code, e.Message)
}

func isSessionExpiredCode(code int) bool {
	switch code {
	case 10, 907, 908:
		return true
	default:
		return false
	}
}

type GetLongPollServerResponse struct {
	Response struct {
		Ts  int `json:"ts"`
		Pts int `json:"pts"`
	} `json:"response"`
}

type Attachment struct {
	Type         string        `json:"type"`
	AudioMessage *AudioMessage `json:"audio_message,omitempty"`
	Doc          *Doc          `json:"doc,omitempty"`
}

type AudioMessage struct {
	ID      int    `json:"id"`
	OwnerID int    `json:"owner_id"`
	LinkOgg string `json:"link_ogg"`
}

type Doc struct {
	Title string `json:"title"`
	Ext   string `json:"ext"`
	URL   string `json:"url"`
}

type Message struct {
	ID          int          `json:"id"`
	FromID      int          `json:"from_id"`
	PeerID      int          `json:"peer_id"`
	Text        string       `json:"text"`
	Date        int          `json:"date"`
	Attachments []Attachment `json:"attachments"`
}

type LongPollHistoryResponse struct {
	Response *struct {
		Messages struct {
			Count int       `json:"count"`
			Items []Message `json:"items"`
		} `json:"messages"`
		NewPts int `json:"new_pts"`
	} `json:"response"`
	Error *struct {
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	} `json:"error"`
}

type SendMessageResponse struct {
	Response int `json:"response"`
	Error    *struct {
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	} `json:"error"`
}

type BotConfig struct {
	Token          string
	ReqBatchSize   int
	ResBatchSize   int
	RequestTimeout time.Duration
	RequestRetries int
	RateLimit      int
}

type VKBotService struct {
	token        string
	apiVersion   string
	lpVersion    int
	groupID      int
	incomingChan chan BotRequestMessage
	outgoingChan chan BotResponseMessage
	httpClient   *http.Client
	retries      int
	semaphore    chan struct{}
}

func NewVKBotService(config BotConfig) *VKBotService {
	return &VKBotService{
		token:        config.Token,
		apiVersion:   "5.199",
		lpVersion:    3,
		groupID:      240715454,
		incomingChan: make(chan BotRequestMessage, config.ReqBatchSize),
		outgoingChan: make(chan BotResponseMessage, config.ResBatchSize),
		httpClient:   &http.Client{Timeout: config.RequestTimeout},
		retries:      config.RequestRetries,
		semaphore:    make(chan struct{}, config.RateLimit),
	}
}

func (s *VKBotService) doRequest(resuest func() (*http.Response, error)) (*http.Response, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
	var resp *http.Response
	var err error
	for r := 0; r <= s.retries; r++ {
		resp, err = resuest()
		if err == nil {
			return resp, nil
		} else {
			logger.Log.Warn("Запрос к API мессенджера завершился ошибкой, повтор", "попытка:", r+1, "error", err.Error())
		}
	}
	return nil, err
}

func (s *VKBotService) Start() {
	go s.listenIncoming()
	go s.listenOutgoing()
}

func (s *VKBotService) SendMessage(message BotResponseMessage) error {
	select {
	case s.outgoingChan <- message:
		logger.Log.Info("Сообщение поставлено в очередь на отправку", "messageID", message.MessageID, "clientID", message.UserID)
		return nil
	default:
		logger.Log.Error("Батч исходящих сообщений переполнен, сообщение отклонено", "messageID", message.MessageID, "clientID", message.UserID)
		return errors.New("Батч исходящих сообщений переполнен")
	}
}

func (s *VKBotService) RecieveMessage() (BotRequestMessage, error) {
	select {
	case msg := <-s.incomingChan:
		return msg, nil
	default:
		return BotRequestMessage{}, errors.New("no incoming messages")
	}
}

func (s *VKBotService) listenIncoming() {
	ts, pts, err := s.getLongPollServer()
	if err != nil {
		logger.Log.Error("Не удалось получить Long Poll сессию", "error", err.Error())
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		historyRes, newPts, err := s.getHistory(ts, pts)
		switch {
		case errors.Is(err, ErrSessionExpired):
			logger.Log.Warn("Long Poll сессия устарела, обновляем", "error", err.Error())
			if ts, pts, err = s.getLongPollServer(); err != nil {
				logger.Log.Error("Не удалось обновить Long Poll сессию", "error", err.Error())
			}
		case err != nil:
			logger.Log.Error("Не удалось получить сообщения", "error", err.Error())
		default:
			s.dispatchIncoming(historyRes.Response.Messages.Items)
			pts = newPts
		}
	}
}

func (s *VKBotService) listenOutgoing() {
	for {
		msg := <-s.outgoingChan
		s.sendMessageToUser(msg)
	}
}

func (s *VKBotService) sendMessageToUser(msg BotResponseMessage) {
	form := url.Values{}
	form.Set("access_token", s.token)
	form.Set("v", s.apiVersion)
	form.Set("user_id", strconv.Itoa(msg.UserID))
	form.Set("message", msg.Text)
	form.Set("random_id", strconv.Itoa(int(rand.Int32())))
	resp, err := s.doRequest(func() (*http.Response, error) {
		return s.httpClient.PostForm("https://api.vk.com/method/messages.send", form)
	})
	if err != nil {
		logger.Log.Error("Не удалось отправить сообщение", "error", err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Log.Error("Не удалось прочитать ответ messages.send", "error", err.Error())
		return
	}
	var result SendMessageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Log.Error("Не удалось разобрать ответ messages.send", "error", err.Error())
		return
	}
	if result.Error != nil {
		apiErr := &APIError{Code: result.Error.ErrorCode, Message: result.Error.ErrorMsg}
		logger.Log.Error("API мессенджера отклонило сообщение", "messageID", msg.MessageID, "clientID", msg.UserID, "error", apiErr.Error())
	}
}

func (s *VKBotService) getLongPollServer() (ts int, pts int, err error) {
	apiURL := fmt.Sprintf(
		"https://api.vk.com/method/messages.getLongPollServer?access_token=%s&v=%s&lp_version=%d&group_id=%d&need_pts=1",
		s.token, s.apiVersion, s.lpVersion, s.groupID,
	)
	resp, err := s.doRequest(func() (*http.Response, error) { return s.httpClient.Get(apiURL) })
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}
	var result GetLongPollServerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, err
	}
	return result.Response.Ts, result.Response.Pts, nil
}

func (s *VKBotService) getHistory(ts int, pts int) (response LongPollHistoryResponse, newPts int, err error) {
	reqURL := fmt.Sprintf(
		"https://api.vk.com/method/messages.getLongPollHistory?access_token=%s&v=%s&ts=%d&pts=%d",
		s.token, s.apiVersion, ts, pts,
	)
	resp, err := s.doRequest(func() (*http.Response, error) { return s.httpClient.Get(reqURL) })
	if err != nil {
		return LongPollHistoryResponse{}, pts, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LongPollHistoryResponse{}, pts, err
	}
	var result LongPollHistoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return LongPollHistoryResponse{}, pts, err
	}
	if result.Error != nil {
		apiErr := &APIError{Code: result.Error.ErrorCode, Message: result.Error.ErrorMsg}
		if isSessionExpiredCode(apiErr.Code) {
			return LongPollHistoryResponse{}, pts, fmt.Errorf("%w: %s", ErrSessionExpired, apiErr)
		}
		return LongPollHistoryResponse{}, pts, apiErr
	}
	if result.Response == nil {
		return LongPollHistoryResponse{}, pts, errors.New("история сообщений пуста")
	}
	if result.Response.NewPts == 0 {
		return result, pts, nil
	}
	return result, result.Response.NewPts, nil
}

func (s *VKBotService) dispatchIncoming(messages []Message) {
	for _, m := range messages {
		pm, err := s.parseIncomingMessage(m)
		if err != nil {
			logger.Log.Error("Пропущено входящее сообщение", "messageID", m.ID, "error", err.Error())
			if errors.Is(err, ErrTooManyAttachments) {
				s.SendMessage(BotResponseMessage{
					MessageID: strconv.Itoa(m.ID),
					UserID:    m.FromID,
					Text:      "Пожалуйста, приложите только один файл.",
					Date:      int(time.Now().Unix()),
				})
			}
			continue
		}
		select {
		case s.incomingChan <- pm:
			logger.Log.Info("Входящее сообщение зарегистрировано", "messageID", pm.MessageID, "clientID", pm.UserID)
		default:
			logger.Log.Error("Батч входящих сообщений переполнен, сообщение отклонено", "messageID", pm.MessageID, "clientID", pm.UserID)
			s.SendMessage(BotResponseMessage{
				MessageID: pm.MessageID,
				UserID:    pm.UserID,
				Text:      "На сервере высокая нагрукза. Попробуйте позже",
				Date:      int(time.Now().Unix()),
			})
		}
	}
}

func (s *VKBotService) parseIncomingMessage(m Message) (BotRequestMessage, error) {
	if m.FromID < 0 {
		return BotRequestMessage{}, errors.New("сообщение от группы или бота")
	}
	if len(m.Attachments) > 1 {
		return BotRequestMessage{}, ErrTooManyAttachments
	}
	for _, a := range m.Attachments {
		switch {
		case a.Type == "audio_message" && a.AudioMessage != nil:
			audioFile, err := s.downloadVoiceMessage(a.AudioMessage)
			if err != nil {
				return BotRequestMessage{}, err
			}
			return BotRequestMessage{
				MessageID: strconv.Itoa(m.ID),
				UserID:    m.FromID,
				AudioFile: audioFile,
				Date:      m.Date,
			}, nil
		case a.Type == "doc" && a.Doc != nil && strings.EqualFold(a.Doc.Ext, "txt"):
			textFile, err := s.downloadTextFile(a.Doc)
			if err != nil {
				return BotRequestMessage{}, err
			}
			return BotRequestMessage{
				MessageID: strconv.Itoa(m.ID),
				UserID:    m.FromID,
				TextFile:  textFile,
				Date:      m.Date,
			}, nil
		}
	}
	if m.Text != "" {
		return BotRequestMessage{
			MessageID: strconv.Itoa(m.ID),
			UserID:    m.FromID,
			Text:      m.Text,
			Date:      m.Date,
		}, nil
	}
	return BotRequestMessage{}, errors.New("сообщение некорректного формата")
}

func (s *VKBotService) downloadVoiceMessage(am *AudioMessage) ([]byte, error) {
	resp, err := s.doRequest(func() (*http.Response, error) { return s.httpClient.Get(am.LinkOgg) })
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (s *VKBotService) downloadTextFile(d *Doc) ([]byte, error) {
	resp, err := s.doRequest(func() (*http.Response, error) { return s.httpClient.Get(d.URL) })
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
