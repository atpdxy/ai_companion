ALTER TABLE scheduled_events ADD COLUMN original_due_at timestamptz;
UPDATE scheduled_events SET original_due_at=due_at;
ALTER TABLE scheduled_events ALTER COLUMN original_due_at SET NOT NULL;
ALTER TABLE scheduled_events ADD COLUMN next_check_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX scheduled_retry ON scheduled_events(next_check_at,due_at) WHERE status='pending';
