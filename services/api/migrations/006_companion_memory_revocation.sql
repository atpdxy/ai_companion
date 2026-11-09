ALTER TABLE companions ADD COLUMN memory_cleared_at timestamptz NOT NULL DEFAULT '-infinity';
