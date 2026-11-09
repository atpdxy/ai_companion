package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"companion/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) ListConversations(ctx context.Context, user string) ([]domain.Conversation, error) {
	return p.listConversations(ctx, user, 200)
}
func (p *Postgres) listConversations(ctx context.Context, user string, limit int) ([]domain.Conversation, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,title,created_at,companion_id FROM conversations WHERE user_id=$1 ORDER BY created_at DESC LIMIT NULLIF($2,0)`, user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Conversation{}
	for rows.Next() {
		var item domain.Conversation
		if err = rows.Scan(&item.ID, &item.Title, &item.CreatedAt, &item.CompanionID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (p *Postgres) CreateConversation(ctx context.Context, user, title string) (domain.Conversation, error) {
	persona, err := p.GetPersona(ctx, user)
	if err != nil {
		return domain.Conversation{}, err
	}
	return p.CreateCompanionConversation(ctx, user, persona.ID, title)
}
func (p *Postgres) Messages(ctx context.Context, user, conversation string) ([]domain.Message, error) {
	var exists bool
	if err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversations WHERE id=$1 AND user_id=$2)`, conversation, user).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrNotFound
	}
	rows, err := p.Pool.Query(ctx, `SELECT m.id,m.turn_id,m.role,m.content,m.created_at FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE c.id=$1 AND c.user_id=$2 ORDER BY m.created_at,m.id`, conversation, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Message{}
	for rows.Next() {
		var item domain.Message
		if err = rows.Scan(&item.ID, &item.TurnID, &item.Role, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (p *Postgres) GetTurn(ctx context.Context, user, conversation, turn string) (domain.Turn, error) {
	item := domain.Turn{ID: turn}
	var result []byte
	err := p.Pool.QueryRow(ctx, `SELECT t.status,t.result,m.id FROM turns t JOIN messages m ON m.turn_id=t.id AND m.role='user' WHERE t.id=$1 AND t.user_id=$2 AND t.conversation_id=$3`, turn, user, conversation).Scan(&item.Status, &result, &item.UserMessageID)
	if err != nil {
		return item, dbError(err)
	}
	if len(result) > 0 {
		item.Result = &domain.AgentResult{}
		if err = json.Unmarshal(result, item.Result); err != nil {
			return item, err
		}
	}
	return item, nil
}
func (p *Postgres) BeginTurn(ctx context.Context, user, conversation, turn, content string) (domain.Turn, error) {
	item := domain.Turn{ID: turn, Status: "processing"}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return item, err
	}
	defer rollback(ctx, tx)
	var owner string
	if err = tx.QueryRow(ctx, `SELECT user_id FROM conversations WHERE id=$1 AND user_id=$2 FOR UPDATE`, conversation, user).Scan(&owner); err != nil {
		return item, dbError(err)
	}
	// A bounded lease allows retry after a crashed API process.
	if _, err = tx.Exec(ctx, `UPDATE turns SET status='incomplete' WHERE conversation_id=$1 AND user_id=$2 AND status='processing' AND updated_at < now()-interval '3 minutes'`, conversation, user); err != nil {
		return item, err
	}
	digest := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(digest[:])
	var storedHash, status string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT content_hash,status,result FROM turns WHERE id=$1 AND user_id=$2 AND conversation_id=$3`, turn, user, conversation).Scan(&storedHash, &status, &raw)
	if err == nil {
		if storedHash != hash {
			return item, domain.ErrConflict
		}
		if status == "processing" {
			return item, domain.ErrConflict
		}
		if err = tx.QueryRow(ctx, `SELECT id FROM messages WHERE turn_id=$1 AND role='user'`, turn).Scan(&item.UserMessageID); err != nil {
			return item, err
		}
		if status == "completed" {
			item.Status = status
			item.Result = &domain.AgentResult{}
			if err = json.Unmarshal(raw, item.Result); err != nil {
				return item, err
			}
			return item, tx.Commit(ctx)
		}
		if _, err = tx.Exec(ctx, `UPDATE turns SET status='processing',updated_at=now() WHERE id=$1 AND user_id=$2`, turn, user); err != nil {
			return item, dbError(err)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err = tx.Exec(ctx, `INSERT INTO turns(id,user_id,conversation_id,content_hash,status) VALUES($1,$2,$3,$4,'processing')`, turn, user, conversation, hash); err != nil {
			return item, dbError(err)
		}
		item.UserMessageID = newID("msg_")
		if _, err = tx.Exec(ctx, `INSERT INTO messages(id,conversation_id,turn_id,role,content) VALUES($1,$2,$3,'user',$4)`, item.UserMessageID, conversation, turn, content); err != nil {
			return item, err
		}
	} else {
		return item, err
	}
	return item, tx.Commit(ctx)
}
func (p *Postgres) FailTurn(ctx context.Context, user, turn string) error {
	_, err := p.Pool.Exec(ctx, `UPDATE turns SET status='incomplete',updated_at=now() WHERE id=$1 AND user_id=$2 AND status='processing'`, turn, user)
	return err
}
func (p *Postgres) FinishTurn(ctx context.Context, user, turn string, result domain.AgentResult) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	var consent bool
	if err = tx.QueryRow(ctx, `SELECT memory_consent FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&consent); err != nil {
		return dbError(err)
	}
	var conversation, source, content, status, companion string
	if err = tx.QueryRow(ctx, `SELECT t.conversation_id,m.id,m.content,t.status,c.companion_id FROM turns t JOIN messages m ON m.turn_id=t.id AND m.role='user' JOIN conversations c ON c.id=t.conversation_id WHERE t.id=$1 AND t.user_id=$2 FOR UPDATE OF t`, turn, user).Scan(&conversation, &source, &content, &status, &companion); err != nil {
		return dbError(err)
	}
	if status != "processing" {
		return domain.ErrConflict
	}
	var afterClear bool
	if err = tx.QueryRow(ctx, `SELECT t.created_at>greatest(u.memory_cleared_at,c.memory_cleared_at) FROM turns t JOIN users u ON u.id=t.user_id JOIN companions c ON c.id=$3 AND c.owner_id=u.id WHERE t.id=$1 AND t.user_id=$2 FOR SHARE OF c`, turn, user, companion).Scan(&afterClear); err != nil {
		return err
	}
	consent = consent && afterClear
	if result.Response == "" {
		return domain.ErrInvalid
	}
	if _, err = tx.Exec(ctx, `INSERT INTO messages(id,conversation_id,turn_id,role,content) VALUES($1,$2,$3,'assistant',$4)`, newID("msg_"), conversation, turn, result.Response); err != nil {
		return err
	}
	if consent {
		for _, candidate := range result.Candidates {
			if candidate.Confidence < .8 || !strings.Contains(content, candidate.SourceQuote) || candidate.SourceQuote == "" {
				continue
			}
			if _, err = tx.Exec(ctx, `INSERT INTO memories(id,user_id,type,content,confidence,source_message_id,source_quote,embedding,companion_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8::vector,$9) ON CONFLICT(user_id,companion_id,content) DO NOTHING`, newID("mem_"), user, candidate.Type, candidate.Content, candidate.Confidence, source, candidate.SourceQuote, lexicalVector(candidate.Content), companion); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO agent_states(user_id,state_json,companion_id) VALUES($1,$2,$3) ON CONFLICT(user_id,companion_id) DO UPDATE SET state_json=EXCLUDED.state_json,updated_at=now()`, user, []byte(result.State), companion); err != nil {
		return err
	}
	// Emotion inference is transient. Never retain it in turn replay or diagnostics.
	result.Emotion = nil
	result.Candidates = nil
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE turns SET status='completed',result=$3,updated_at=now() WHERE id=$1 AND user_id=$2`, turn, user, raw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) GetState(ctx context.Context, user string) (json.RawMessage, error) {
	persona, err := p.GetPersona(ctx, user)
	if err != nil {
		return nil, err
	}
	return p.GetCompanionState(ctx, user, persona.ID)
}
func (p *Postgres) GetCompanionState(ctx context.Context, user, companion string) (json.RawMessage, error) {
	var raw []byte
	err := p.Pool.QueryRow(ctx, `SELECT state_json FROM agent_states WHERE user_id=$1 AND companion_id=$2`, user, companion).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return json.RawMessage(`{"valence":0.15,"arousal":0.25,"familiarity":0,"interaction_energy":0.8}`), nil
	}
	return raw, err
}
func (p *Postgres) Feedback(ctx context.Context, user, message string, rating int, tag string) error {
	var exists bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE m.id=$1 AND c.user_id=$2 AND m.role='assistant')`, message, user).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrNotFound
	}
	_, err = p.Pool.Exec(ctx, `INSERT INTO feedback(id,user_id,message_id,rating,tag) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,message_id) DO UPDATE SET rating=EXCLUDED.rating,tag=EXCLUDED.tag`, newID("fb_"), user, message, rating, tag)
	return err
}
