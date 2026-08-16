CREATE TABLE meetings (
    meeting_id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id),
    message_id VARCHAR(255),
    audio_file BYTEA,
    text_file BYTEA,
    transcription_text TEXT,
    summary_text TEXT,
    request_text TEXT,
    response_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_meetings_user_id ON meetings(user_id);
