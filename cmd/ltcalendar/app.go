package main

import (
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
	repo         repository.RepositoryInterface
	bot          bot.BotInterface
	speechClient speech.SpeechClientInterface
	llmClient    llm.LLMClientInterface
	cfg          config.Config
}

func NewApp(repo repository.RepositoryInterface, bot bot.BotInterface, speechClient speech.SpeechClientInterface, llmClient llm.LLMClientInterface, cfg config.Config) *App {
	return &App{
		repo:         repo,
		bot:          bot,
		speechClient: speechClient,
		llmClient:    llmClient,
		cfg:          cfg,
	}
}

func (a *App) GetHandler() http.Handler {
	appService := service.NewAppService(a.repo, a.bot, a.speechClient, a.llmClient, &a.cfg)
	requestsHandler := handler.NewRequestsHandler(appService)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", requestsHandler.Ping)
	return mux
}

func (a *App) StartBot() {
	a.bot.Start()
}
