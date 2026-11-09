CREATE TABLE character_cards (
    id text PRIMARY KEY,
    owner_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    definition jsonb NOT NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(id,owner_id)
);
CREATE TABLE companions (
    id text PRIMARY KEY,
    owner_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id text NOT NULL,
    config jsonb NOT NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY(card_id,owner_id) REFERENCES character_cards(id,owner_id),
    UNIQUE(id,owner_id)
);
INSERT INTO character_cards(id,owner_id,definition)
SELECT 'card_'||u.id||'_lover',u.id,
    p.config || '{"relationship":"lover","description":"温柔、真诚的虚拟爱人，尊重彼此的生活与边界。","greeting":"今天的你，想从哪里说起？","voice_id":""}'::jsonb
FROM users u JOIN personas p ON p.owner_id=u.id;
INSERT INTO character_cards(id,owner_id,definition)
SELECT 'card_'||u.id||'_'||r.role,u.id,
    p.config || jsonb_build_object('name',r.name,'relationship',r.role,'description',r.description,'greeting',r.greeting,'voice_id','')
FROM users u JOIN personas p ON p.owner_id=u.id CROSS JOIN (VALUES
    ('friend','林舟','坦诚、幽默的虚拟朋友，平等交流，支持现实社交。','最近怎么样？说说你的新鲜事。'),
    ('family','知秋','温和、关怀的虚拟家人，以亲情方式陪伴，不扮演爱人。','回来了，今天过得怎么样？')
) AS r(role,name,description,greeting);
INSERT INTO companions(id,owner_id,card_id,config)
SELECT 'cmp_'||owner_id||'_default',owner_id,id,definition FROM character_cards WHERE id='card_'||owner_id||'_lover';
ALTER TABLE users ADD COLUMN active_companion_id text;
UPDATE users SET active_companion_id='cmp_'||id||'_default';
ALTER TABLE users ADD CONSTRAINT active_companion_owner FOREIGN KEY(active_companion_id,id) REFERENCES companions(id,owner_id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE conversations ADD COLUMN companion_id text;
UPDATE conversations c SET companion_id=u.active_companion_id FROM users u WHERE u.id=c.user_id;
ALTER TABLE conversations ALTER COLUMN companion_id SET NOT NULL;
ALTER TABLE conversations ADD FOREIGN KEY(companion_id,user_id) REFERENCES companions(id,owner_id) ON DELETE CASCADE;
ALTER TABLE memories ADD COLUMN companion_id text;
UPDATE memories m SET companion_id=c.companion_id FROM messages s JOIN conversations c ON c.id=s.conversation_id WHERE s.id=m.source_message_id;
ALTER TABLE memories ALTER COLUMN companion_id SET NOT NULL;
ALTER TABLE memories ADD FOREIGN KEY(companion_id,user_id) REFERENCES companions(id,owner_id) ON DELETE CASCADE;
ALTER TABLE memories DROP CONSTRAINT memories_user_id_content_key;
ALTER TABLE memories ADD UNIQUE(user_id,companion_id,content);
CREATE INDEX memories_companion ON memories(user_id,companion_id,consent_status);
ALTER TABLE agent_states ADD COLUMN companion_id text;
UPDATE agent_states s SET companion_id=u.active_companion_id FROM users u WHERE u.id=s.user_id;
ALTER TABLE agent_states ALTER COLUMN companion_id SET NOT NULL;
ALTER TABLE agent_states DROP CONSTRAINT agent_states_pkey;
ALTER TABLE agent_states ADD PRIMARY KEY(user_id,companion_id);
ALTER TABLE agent_states ADD FOREIGN KEY(companion_id,user_id) REFERENCES companions(id,owner_id) ON DELETE CASCADE;
