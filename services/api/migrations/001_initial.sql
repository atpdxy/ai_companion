CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE users (
    id text PRIMARY KEY,
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    memory_consent boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE personas (
    owner_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    config jsonb NOT NULL,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE TABLE conversations (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(id, user_id)
);
CREATE INDEX conversations_owner ON conversations(user_id, created_at DESC);
CREATE TABLE turns (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id text NOT NULL,
    content_hash text NOT NULL,
    status text NOT NULL CHECK (status IN ('processing','completed','incomplete')),
    result jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY(conversation_id,user_id) REFERENCES conversations(id,user_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX one_active_turn ON turns(conversation_id) WHERE status='processing';
CREATE TABLE messages (
    id text PRIMARY KEY,
    conversation_id text NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    turn_id text NOT NULL REFERENCES turns(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user','assistant')),
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(turn_id,role)
);
CREATE INDEX messages_timeline ON messages(conversation_id, created_at, id);
CREATE TABLE memories (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('semantic','episodic','boundary')),
    content text NOT NULL,
    confidence double precision NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    importance double precision NOT NULL DEFAULT 0.5 CHECK (importance BETWEEN 0 AND 1),
    consent_status text NOT NULL DEFAULT 'pending' CHECK (consent_status IN ('pending','approved','rejected')),
    source_message_id text NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    source_quote text NOT NULL,
    embedding vector(128) NOT NULL,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id,content)
);
CREATE INDEX memories_owner ON memories(user_id,consent_status,created_at DESC);
CREATE INDEX memories_embedding ON memories USING hnsw (embedding vector_cosine_ops);
CREATE TABLE agent_states (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    state_json jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE feedback (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_id text NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    rating integer NOT NULL CHECK (rating IN (-1,1)),
    tag text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id,message_id)
);
CREATE TABLE audit_logs (
    id text PRIMARY KEY,
    actor_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action text NOT NULL,
    object_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_owner ON audit_logs(actor_id,created_at DESC);
