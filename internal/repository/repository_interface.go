package repository

import (
	"context"
	"errors"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/model"
)

var ErrDBHostWrongFormat = errors.New("database host/port wrong format")
var ErrMeetingNotFound = errors.New("meeting not found")

type RepositoryInterface interface {
	Ping(ctx context.Context) error
	CreateUser(ctx context.Context, userID int) error
	CreateMeeting(ctx context.Context, meeting model.Meeting) (int, error)
	UpdateMeeting(ctx context.Context, meeting model.Meeting) error
	UpdateMeetingStatus(ctx context.Context, meetingID int, status model.ProcessingStatus, errorText string) error
	GetMeeting(ctx context.Context, userID int, meetingID int) (model.Meeting, error)
	ListMeetings(ctx context.Context, userID int) ([]model.Meeting, error)
	FindMeetings(ctx context.Context, userID int, keyword string) ([]model.Meeting, error)
	DeleteMeeting(ctx context.Context, userID int, meetingID int) error
}
