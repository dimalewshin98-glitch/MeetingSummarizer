package config

import (
	"flag"
	"os"
	"strconv"
)

type Config struct {
	ServerHostPort          string
	LogLevel                string
	DatabaseDsn             string
	SecretKey               string
	BotToken                string
	BotReqBatchSize         int
	BotResBatchSize         int
	BotRequestTimeoutSec    int
	BotRequestRetries       int
	BotRequestRateLimit     int
	LLMRequestTimeoutSec    int
	LLMRequestRetries       int
	LLMRequestRateLimit     int
	SpeechRequestTimeoutSec int
	SpeechRequestRetries    int
	SpeechRequestRateLimit  int
	SpeechProvider          string
	LLMProvider             string
	WorkersCount            int
	LLMAuthKey              string
}

func NewConfig() *Config {
	serverHostPort := flag.String("a", "localhost:8888", "server host:port")
	logLevel := flag.String("l", "info", "log level")
	databaseDsn := flag.String("d", "localhost:5432", "databse destination (host/host:port)")
	secretKey := flag.String("sk", "", "Secret key")
	botToken := flag.String("bt", "", "Bot auth token")
	botReqBatchSize := flag.Int("brb", 10, "Bot incoming messages batch size")
	botResBatchSize := flag.Int("bsb", 10, "Bot outgoing messages batch size")
	botRequestTimeoutSec := flag.Int("brt", 10, "Bot outgoing requests timeout, seconds")
	botRequestRetries := flag.Int("brr", 2, "Bot outgoing requests retry count")
	botRequestRateLimit := flag.Int("brl", 5, "Bot outgoing requests max concurrency")
	llmRequestTimeoutSec := flag.Int("lrt", 10, "LLM client requests timeout, seconds")
	llmRequestRetries := flag.Int("lrr", 2, "LLM client requests retry count")
	llmRequestRateLimit := flag.Int("lrl", 5, "LLM client requests max concurrency")
	speechRequestTimeoutSec := flag.Int("srt", 10, "Speech client requests timeout, seconds")
	speechRequestRetries := flag.Int("srr", 2, "Speech client requests retry count")
	speechRequestRateLimit := flag.Int("srl", 5, "Speech client requests max concurrency")
	speechProvider := flag.String("sp", "mock", "Speech client provider (mock)")
	llmProvider := flag.String("lp", "gigachat", "LLM client provider (mock, gigachat)")
	workersCount := flag.Int("wc", 5, "Meeting processing workers count")
	llmAuthKey := flag.String("lak", "", "llmAuthKey")
	flag.Parse()
	if envServerHostPort := os.Getenv("SERVER_ADDRESS"); envServerHostPort != "" {
		*serverHostPort = envServerHostPort
	}
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		*logLevel = envLogLevel
	}
	if envDatabaseDsn := os.Getenv("DATABASE_DSN"); envDatabaseDsn != "" {
		*databaseDsn = envDatabaseDsn
	}
	if envSecretKey := os.Getenv("SECRET_KEY"); envSecretKey != "" {
		*secretKey = envSecretKey
	}
	if envBotToken := os.Getenv("BOT_TOKEN"); envBotToken != "" {
		*botToken = envBotToken
	}
	if envBotReqBatchSize := os.Getenv("BOT_REQ_BATCH_SIZE"); envBotReqBatchSize != "" {
		envBotReqBatchSizeInt, err := strconv.Atoi(envBotReqBatchSize)
		if err != nil {
		} else {
			*botReqBatchSize = envBotReqBatchSizeInt
		}
	}
	if envBotResBatchSize := os.Getenv("BOT_RES_BATCH_SIZE"); envBotResBatchSize != "" {
		envBotResBatchSizeInt, err := strconv.Atoi(envBotResBatchSize)
		if err != nil {
		} else {
			*botResBatchSize = envBotResBatchSizeInt
		}
	}
	if envBotRequestTimeoutSec := os.Getenv("BOT_REQUEST_TIMEOUT_SEC"); envBotRequestTimeoutSec != "" {
		envBotRequestTimeoutSecInt, err := strconv.Atoi(envBotRequestTimeoutSec)
		if err != nil {
		} else {
			*botRequestTimeoutSec = envBotRequestTimeoutSecInt
		}
	}
	if envBotRequestRetries := os.Getenv("BOT_REQUEST_RETRIES"); envBotRequestRetries != "" {
		envBotRequestRetriesInt, err := strconv.Atoi(envBotRequestRetries)
		if err != nil {
		} else {
			*botRequestRetries = envBotRequestRetriesInt
		}
	}
	if envBotRequestRateLimit := os.Getenv("BOT_REQUEST_RATE_LIMIT"); envBotRequestRateLimit != "" {
		envBotRequestRateLimitInt, err := strconv.Atoi(envBotRequestRateLimit)
		if err != nil {
		} else {
			*botRequestRateLimit = envBotRequestRateLimitInt
		}
	}
	if envLLMRequestTimeoutSec := os.Getenv("LLM_REQUEST_TIMEOUT_SEC"); envLLMRequestTimeoutSec != "" {
		envLLMRequestTimeoutSecInt, err := strconv.Atoi(envLLMRequestTimeoutSec)
		if err != nil {
		} else {
			*llmRequestTimeoutSec = envLLMRequestTimeoutSecInt
		}
	}
	if envLLMRequestRetries := os.Getenv("LLM_REQUEST_RETRIES"); envLLMRequestRetries != "" {
		envLLMRequestRetriesInt, err := strconv.Atoi(envLLMRequestRetries)
		if err != nil {
		} else {
			*llmRequestRetries = envLLMRequestRetriesInt
		}
	}
	if envLLMRequestRateLimit := os.Getenv("LLM_REQUEST_RATE_LIMIT"); envLLMRequestRateLimit != "" {
		envLLMRequestRateLimitInt, err := strconv.Atoi(envLLMRequestRateLimit)
		if err != nil {
		} else {
			*llmRequestRateLimit = envLLMRequestRateLimitInt
		}
	}
	if envSpeechRequestTimeoutSec := os.Getenv("SPEECH_REQUEST_TIMEOUT_SEC"); envSpeechRequestTimeoutSec != "" {
		envSpeechRequestTimeoutSecInt, err := strconv.Atoi(envSpeechRequestTimeoutSec)
		if err != nil {
		} else {
			*speechRequestTimeoutSec = envSpeechRequestTimeoutSecInt
		}
	}
	if envSpeechRequestRetries := os.Getenv("SPEECH_REQUEST_RETRIES"); envSpeechRequestRetries != "" {
		envSpeechRequestRetriesInt, err := strconv.Atoi(envSpeechRequestRetries)
		if err != nil {
		} else {
			*speechRequestRetries = envSpeechRequestRetriesInt
		}
	}
	if envSpeechRequestRateLimit := os.Getenv("SPEECH_REQUEST_RATE_LIMIT"); envSpeechRequestRateLimit != "" {
		envSpeechRequestRateLimitInt, err := strconv.Atoi(envSpeechRequestRateLimit)
		if err != nil {
		} else {
			*speechRequestRateLimit = envSpeechRequestRateLimitInt
		}
	}
	if envSpeechProvider := os.Getenv("SPEECH_PROVIDER"); envSpeechProvider != "" {
		*speechProvider = envSpeechProvider
	}
	if envLLMProvider := os.Getenv("LLM_PROVIDER"); envLLMProvider != "" {
		*llmProvider = envLLMProvider
	}
	if envLLMAuthKey := os.Getenv("LLM_AUTH_KEY"); envLLMAuthKey != "" {
		*llmAuthKey = envLLMAuthKey
	}
	if envWorkersCount := os.Getenv("WORKERS_COUNT"); envWorkersCount != "" {
		envWorkersCountInt, err := strconv.Atoi(envWorkersCount)
		if err != nil {
		} else {
			*workersCount = envWorkersCountInt
		}
	}
	conf := &Config{
		ServerHostPort:          *serverHostPort,
		LogLevel:                *logLevel,
		DatabaseDsn:             *databaseDsn,
		SecretKey:               *secretKey,
		BotToken:                *botToken,
		BotReqBatchSize:         *botReqBatchSize,
		BotResBatchSize:         *botResBatchSize,
		BotRequestTimeoutSec:    *botRequestTimeoutSec,
		BotRequestRetries:       *botRequestRetries,
		BotRequestRateLimit:     *botRequestRateLimit,
		LLMRequestTimeoutSec:    *llmRequestTimeoutSec,
		LLMRequestRetries:       *llmRequestRetries,
		LLMRequestRateLimit:     *llmRequestRateLimit,
		SpeechRequestTimeoutSec: *speechRequestTimeoutSec,
		SpeechRequestRetries:    *speechRequestRetries,
		SpeechRequestRateLimit:  *speechRequestRateLimit,
		SpeechProvider:          *speechProvider,
		LLMProvider:             *llmProvider,
		WorkersCount:            *workersCount,
		LLMAuthKey:              *llmAuthKey,
	}
	return conf
}
