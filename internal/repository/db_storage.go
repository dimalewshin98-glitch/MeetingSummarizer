package repository

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/000001_create_users_table.up.sql
var sqlCreateUsersTable string

//go:embed migrations/000002_create_meetings_table.up.sql
var sqlCreateMeetingsTable string

//go:embed migrations/000003_create_meeting_status_history_table.up.sql
var sqlCreateMeetingStatusHistoryTable string

type DBRepository struct {
	dbDsn        string
	dbConnection *sql.DB
}

func NewDBRepository(dbDsn string) (*DBRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hostPort := strings.Split(dbDsn, ":")
	var ps string
	if len(hostPort) == 1 {
		ps = fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
			hostPort[0], `postgres`, `admin`, `postgres`)
	} else if len(hostPort) == 2 {
		ps = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			hostPort[0], hostPort[1], `postgres`, `admin`, `postgres`)
	} else {
		return nil, ErrDBHostWrongFormat
	}
	db, err := sql.Open("pgx", ps)
	if err != nil {
		return nil, err
	}
	dbRepository := &DBRepository{
		dbDsn:        dbDsn,
		dbConnection: db,
	}
	if err := dbRepository.Ping(ctx); err != nil {
		return nil, err
	}
	if err := dbRepository.CreateTables(ctx); err != nil {
		return nil, err
	}
	return dbRepository, nil
}

func (r *DBRepository) Ping(ctx context.Context) error {
	return r.dbConnection.PingContext(ctx)
}

func (r *DBRepository) CreateTables(ctx context.Context) error {
	tx, err := r.dbConnection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	sqlReq := "SELECT table_name FROM information_schema.tables WHERE table_schema='public';"
	rows, err := tx.QueryContext(ctx, sqlReq)
	if err != nil {
		return nil
	}
	var tableName string
	var tables []string
	for rows.Next() {
		rows.Scan(&tableName)
		tables = append(tables, tableName)
	}
	if !slices.Contains(tables, "users") {
		_, err = tx.ExecContext(ctx, sqlCreateUsersTable)
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return rbErr
			}
			return err
		}
	}
	if !slices.Contains(tables, "meetings") {
		_, err = tx.ExecContext(ctx, sqlCreateMeetingsTable)
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return rbErr
			}
			return err
		}
	}
	if !slices.Contains(tables, "meeting_status_history") {
		_, err = tx.ExecContext(ctx, sqlCreateMeetingStatusHistoryTable)
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return rbErr
			}
			return err
		}
	}
	return tx.Commit()
}

func (r *DBRepository) CreateUser(ctx context.Context, userID int) error {
	_, err := r.dbConnection.ExecContext(ctx,
		`INSERT INTO users (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}

func (r *DBRepository) CreateMeeting(ctx context.Context, meeting model.Meeting) (int, error) {
	tx, err := r.dbConnection.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	var meetingID int
	err = tx.QueryRowContext(ctx,
		`INSERT INTO meetings (user_id, message_id, audio_file, text_file, transcription_text, summary_text, request_text, response_text)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING meeting_id`,
		meeting.UserID, meeting.MessageID, meeting.AudioFile, meeting.TextFile, meeting.TranscriptionText, meeting.SummaryText, meeting.RequestText, meeting.ResponseText,
	).Scan(&meetingID)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return 0, rbErr
		}
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO meeting_status_history (meeting_id, status, error_text) VALUES ($1, $2, $3)`,
		meetingID, meeting.Status, meeting.ErrorText,
	); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return 0, rbErr
		}
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return meetingID, nil
}

func (r *DBRepository) GetMeeting(ctx context.Context, userID int, meetingID int) (model.Meeting, error) {
	var meeting model.Meeting
	row := r.dbConnection.QueryRowContext(ctx,
		`SELECT m.meeting_id, m.user_id, m.message_id, m.audio_file, m.text_file, m.transcription_text, m.summary_text, m.request_text, m.response_text, m.created_at,
		        h.status, h.error_text, h.created_at
		 FROM meetings m
		 JOIN meeting_status_history h ON h.meeting_id = m.meeting_id
		 WHERE m.meeting_id = $1 AND m.user_id = $2
		 ORDER BY h.id DESC
		 LIMIT 1`,
		meetingID, userID)
	err := row.Scan(&meeting.MeetingID, &meeting.UserID, &meeting.MessageID, &meeting.AudioFile, &meeting.TextFile, &meeting.TranscriptionText, &meeting.SummaryText, &meeting.RequestText, &meeting.ResponseText, &meeting.CreatedAt, &meeting.Status, &meeting.ErrorText, &meeting.StatusUpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Meeting{}, ErrMeetingNotFound
		}
		return model.Meeting{}, err
	}
	return meeting, nil
}

func (r *DBRepository) ListMeetings(ctx context.Context, userID int) ([]model.Meeting, error) {
	rows, err := r.dbConnection.QueryContext(ctx,
		`SELECT m.meeting_id, m.user_id, m.message_id, m.audio_file, m.text_file, m.transcription_text, m.summary_text, m.request_text, m.response_text, m.created_at,
		        h.status, h.error_text, h.created_at
		 FROM meetings m JOIN (
			SELECT DISTINCT ON (meeting_id) meeting_id, status, error_text, created_at
			FROM meeting_status_history
			ORDER BY meeting_id, id DESC
		 ) h ON h.meeting_id = m.meeting_id
		 WHERE m.user_id = $1
		 ORDER BY m.meeting_id DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanMeetings(rows)
}

func (r *DBRepository) FindMeetings(ctx context.Context, userID int, keyword string) ([]model.Meeting, error) {
	pattern := "%" + keyword + "%"
	rows, err := r.dbConnection.QueryContext(ctx,
		`SELECT m.meeting_id, m.user_id, m.message_id, m.audio_file, m.text_file, m.transcription_text, m.summary_text, m.request_text, m.response_text, m.created_at,
		        h.status, h.error_text, h.created_at
		 FROM meetings m JOIN (
			SELECT DISTINCT ON (meeting_id) meeting_id, status, error_text, created_at
			FROM meeting_status_history
			ORDER BY meeting_id, id DESC
		 ) h ON h.meeting_id = m.meeting_id
		 WHERE m.user_id = $1 AND (m.transcription_text ILIKE $2 OR m.summary_text ILIKE $2)
		 ORDER BY m.meeting_id DESC`,
		userID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanMeetings(rows)
}

func (r *DBRepository) UpdateMeeting(ctx context.Context, meeting model.Meeting) error {
	tx, err := r.dbConnection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE meetings SET message_id = $1, audio_file = $2, text_file = $3, transcription_text = $4, summary_text = $5, request_text = $6, response_text = $7
		 WHERE meeting_id = $8`,
		meeting.MessageID, meeting.AudioFile, meeting.TextFile, meeting.TranscriptionText, meeting.SummaryText, meeting.RequestText, meeting.ResponseText, meeting.MeetingID,
	); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO meeting_status_history (meeting_id, status, error_text) VALUES ($1, $2, $3)`,
		meeting.MeetingID, meeting.Status, meeting.ErrorText,
	); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}
	return tx.Commit()
}

func (r *DBRepository) UpdateMeetingStatus(ctx context.Context, meetingID int, status model.ProcessingStatus, errorText string) error {
	_, err := r.dbConnection.ExecContext(ctx,
		`INSERT INTO meeting_status_history (meeting_id, status, error_text) VALUES ($1, $2, $3)`,
		meetingID, status, errorText)
	return err
}

func (r *DBRepository) DeleteMeeting(ctx context.Context, userID int, meetingID int) error {
	tx, err := r.dbConnection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM meeting_status_history
		 WHERE meeting_id = $1 AND EXISTS (SELECT 1 FROM meetings WHERE meeting_id = $1 AND user_id = $2)`,
		meetingID, userID,
	); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}
	_, err = tx.ExecContext(ctx,
		`DELETE FROM meetings WHERE meeting_id = $1 AND user_id = $2`, meetingID, userID)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}
	return tx.Commit()
}

func (r *DBRepository) scanMeetings(rows *sql.Rows) ([]model.Meeting, error) {
	var meetings []model.Meeting
	for rows.Next() {
		var meeting model.Meeting
		if err := rows.Scan(&meeting.MeetingID, &meeting.UserID, &meeting.MessageID, &meeting.AudioFile, &meeting.TextFile, &meeting.TranscriptionText, &meeting.SummaryText, &meeting.RequestText, &meeting.ResponseText, &meeting.CreatedAt, &meeting.Status, &meeting.ErrorText, &meeting.StatusUpdatedAt); err != nil {
			return nil, err
		}
		meetings = append(meetings, meeting)
	}
	return meetings, rows.Err()
}
