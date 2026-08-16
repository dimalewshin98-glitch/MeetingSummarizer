package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/bot"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/config"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/handler"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/llm"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/logger"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/repository"
	"github.com/dimalewshin98-glitch/LTCalendar/internal/speech"
)

func main() {
	cfg := config.NewConfig()
	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic(err)
	}

	repo, err := repository.NewDBRepository(cfg.DatabaseDsn)
	if err != nil {
		panic(err)
	}
	logger.Log.Info("Repository address set to", "dsn", cfg.DatabaseDsn)

	messengerBot := bot.NewVKBotService(bot.BotConfig{
		Token:          cfg.BotToken,
		ReqBatchSize:   cfg.BotReqBatchSize,
		ResBatchSize:   cfg.BotResBatchSize,
		RequestTimeout: time.Duration(cfg.BotRequestTimeoutSec) * time.Second,
		RequestRetries: cfg.BotRequestRetries,
		RateLimit:      cfg.BotRequestRateLimit,
	})

	speechClient, err := speech.NewSpeechClient(cfg.SpeechProvider, speech.SpeechClientConfig{
		RequestTimeout: time.Duration(cfg.SpeechRequestTimeoutSec) * time.Second,
		RequestRetries: cfg.SpeechRequestRetries,
		RateLimit:      cfg.SpeechRequestRateLimit,
	})
	if err != nil {
		panic(err)
	}
	llmClient, err := llm.NewLLMClient(cfg.LLMProvider, llm.LLMClientConfig{
		RequestTimeout: time.Duration(cfg.LLMRequestTimeoutSec) * time.Second,
		RequestRetries: cfg.LLMRequestRetries,
		RateLimit:      cfg.LLMRequestRateLimit,
	})
	if err != nil {
		panic(err)
	}

	app := NewApp(repo, messengerBot, speechClient, llmClient, *cfg)
	app.StartBot()
	app.StartWorkers()

	appHandler := app.GetHandler()
	var srv = http.Server{Addr: cfg.ServerHostPort, Handler: logger.RequestLogger(handler.AuthMiddleware(handler.GzipMiddleware(appHandler), repo, cfg.SecretKey))}
	idleConnsClosed := make(chan struct{})
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		app.ShutdownWorkers()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Log.Error("HTTP server Shutdown", "error", err.Error())
		}
		if err := repo.Close(ctx); err != nil {
			logger.Log.Error("Repo Shutdown", "error", err.Error())
		}
		close(idleConnsClosed)
	}()
	logger.Log.Info("Running server", "address", cfg.ServerHostPort)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Log.Error("Server failed", "error", err)
		panic(err)
	}

	<-idleConnsClosed
	logger.Log.Info("Service Shutdown gracefully", "address", cfg.ServerHostPort)
}
