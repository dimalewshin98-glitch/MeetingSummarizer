CREATE TABLE meeting_status_history (
    id SERIAL PRIMARY KEY,
    meeting_id INTEGER NOT NULL REFERENCES meetings(meeting_id),
    status VARCHAR(32) NOT NULL,
    error_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE INDEX idx_meeting_status_history_meeting_id ON meeting_status_history(meeting_id);
CREATE INDEX idx_meeting_status_history_id ON meeting_status_history(id);