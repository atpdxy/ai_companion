CREATE TABLE notification_prefs (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    opt_in boolean NOT NULL DEFAULT false,
    no_contact boolean NOT NULL DEFAULT false,
    quiet_start integer NOT NULL DEFAULT 22 CHECK (quiet_start BETWEEN 0 AND 23),
    quiet_end integer NOT NULL DEFAULT 8 CHECK (quiet_end BETWEEN 0 AND 23),
    daily_limit integer NOT NULL DEFAULT 3 CHECK (daily_limit BETWEEN 1 AND 10),
    timezone text NOT NULL DEFAULT 'Asia/Shanghai'
);
INSERT INTO notification_prefs(user_id) SELECT id FROM users;
CREATE TABLE scheduled_events (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content text NOT NULL,
    due_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    recurrence text NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','daily','weekly')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','cancelled','delivered','expired')),
    client_key text NOT NULL,
    UNIQUE(user_id,client_key)
);
CREATE INDEX scheduled_due ON scheduled_events(due_at) WHERE status='pending';
CREATE TABLE notifications (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id text NOT NULL REFERENCES scheduled_events(id) ON DELETE CASCADE,
    occurrence_at timestamptz NOT NULL,
    content text NOT NULL,
    delivered_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(event_id,occurrence_at)
);
CREATE INDEX notification_owner ON notifications(user_id,delivered_at DESC);
