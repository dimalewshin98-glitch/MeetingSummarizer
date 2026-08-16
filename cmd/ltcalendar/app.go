package main

import (
	"context"
	"net/http"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/bot"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/config"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/handler"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/llm"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/repository"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/service"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/speech"
)

type App struct {
	service       *service.AppService
	repo          repository.RepositoryInterface
	bot           bot.BotInterface
	speechClient  speech.SpeechClientInterface
	llmClient     llm.LLMClientInterface
	cfg           config.Config
	workersCancel context.CancelFunc
}

func NewApp(repo repository.RepositoryInterface, bot bot.BotInterface, speechClient speech.SpeechClientInterface, llmClient llm.LLMClientInterface, cfg config.Config) *App {
	return &App{
		service:      service.NewAppService(repo, bot, speechClient, llmClient, &cfg),
		repo:         repo,
		bot:          bot,
		speechClient: speechClient,
		llmClient:    llmClient,
		cfg:          cfg,
	}
}

func (a *App) GetHandler() http.Handler {
	requestsHandler := handler.NewRequestsHandler(a.service)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", requestsHandler.Ping)
	return mux
}

func (a *App) StartBot() {
	a.bot.Start()
}

func (a *App) StartWorkers() {
	var ctx context.Context
	ctx, a.workersCancel = context.WithCancel(context.Background())
	a.service.StartWorkers(ctx)
	a.service.StartBotListener(ctx)
}

func (a *App) ShutdownWorkers() {
	if a.workersCancel != nil {
		a.workersCancel()
	}
}
