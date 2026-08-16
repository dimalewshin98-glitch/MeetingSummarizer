package model

import "time"

type ProcessingStatus string

const (
	StatusCreated     ProcessingStatus = "created"
	StatusProcessing  ProcessingStatus = "processing"
	StatusTranscribed ProcessingStatus = "transcribed"
	StatusSummarized  ProcessingStatus = "summarized"
	StatusCompleted   ProcessingStatus = "completed"
	StatusFailed      ProcessingStatus = "failed"
)

type User struct {
	UserID int
}

type Meeting struct {
	MeetingID         int
	UserID            int
	MessageID         string
	AudioFile         []byte
	TextFile          []byte
	TranscriptionText string
	SummaryText       string
	RequestText       string
	ResponseText      string
	Status            ProcessingStatus
	ErrorText         string
	CreatedAt         time.Time
	StatusUpdatedAt   time.Time
}
