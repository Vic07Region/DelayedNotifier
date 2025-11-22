CREATE TYPE notification_status AS ENUM (
    'pending',
    'processing',
    'sent',
    'failed',
    'cancelled'
);

-- Основная таблица
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient TEXT NOT NULL,
    channel TEXT NOT NULL, -- "email", "telegram"
    payload JSONB NOT NULL,
    scheduled_at TIMESTAMPTZ NOT NULL,
    status notification_status NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_pending_scheduled
    ON notifications (scheduled_at)
    WHERE status = 'pending';


CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();