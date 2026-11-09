ALTER TABLE users ADD COLUMN memory_cleared_at timestamptz NOT NULL DEFAULT '-infinity';
ALTER TABLE turns ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
