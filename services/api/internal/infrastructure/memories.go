package infrastructure

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"companion/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// A deterministic character-bigram feature vector, NOT a neural semantic embedding.
// Version 1 intentionally has no additional model dependency. See docs/implementation.md.
func lexicalVector(text string) string {
	var values [128]float64
	runes := []rune(strings.ToLower(text))
	for i := 0; i < len(runes); i++ {
		end := i + 2
		if end > len(runes) {
			end = len(runes)
		}
		h := fnv.New32a()
		if _, err := h.Write([]byte(string(runes[i:end]))); err != nil {
			panic(err)
		}
		values[h.Sum32()%128]++
	}
	var norm float64
	for _, v := range values {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		norm = 1
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.FormatFloat(v/norm, 'f', 6, 64)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
func scanMemory(row pgx.Row) (domain.Memory, error) {
	var m domain.Memory
	err := row.Scan(&m.CompanionID, &m.ID, &m.Content, &m.Type, &m.ConsentStatus, &m.Confidence, &m.Importance, &m.SourceMessageID, &m.SourceQuote, &m.ExpiresAt, &m.CreatedAt)
	return m, dbError(err)
}

const memoryColumns = `companion_id,id,content,type,consent_status,confidence,importance,source_message_id,source_quote,expires_at,created_at`

func (p *Postgres) ListMemories(ctx context.Context, user string) ([]domain.Memory, error) {
	persona, err := p.GetPersona(ctx, user)
	if err != nil {
		return nil, err
	}
	return p.CompanionMemories(ctx, user, persona.ID)
}
func (p *Postgres) listMemories(ctx context.Context, user string, limit int) ([]domain.Memory, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+memoryColumns+` FROM memories WHERE user_id=$1 ORDER BY created_at DESC LIMIT NULLIF($2,0)`, user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Memory{}
	for rows.Next() {
		m, scanErr := scanMemory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (p *Postgres) UpdateMemory(ctx context.Context, user, id string, update domain.MemoryUpdate) (domain.Memory, error) {
	var m domain.Memory
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return m, err
	}
	defer rollback(ctx, tx)
	// The vector and canonical content change in one PostgreSQL transaction.
	m, err = scanMemory(tx.QueryRow(ctx, `UPDATE memories SET content=$3,consent_status=$4,expires_at=$5,embedding=$6::vector,updated_at=now(),confidence=1 WHERE id=$1 AND user_id=$2 RETURNING `+memoryColumns, id, user, update.Content, update.ConsentStatus, update.ExpiresAt, lexicalVector(update.Content)))
	if err != nil {
		return m, err
	}
	if err = audit(ctx, tx, user, "memory_updated", id); err != nil {
		return m, err
	}
	return m, tx.Commit(ctx)
}
func (p *Postgres) DeleteMemory(ctx context.Context, user, id string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `DELETE FROM memories WHERE id=$1 AND user_id=$2`, id, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err = audit(ctx, tx, user, "memory_deleted", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) DeleteMemories(ctx context.Context, user string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	if _, err = tx.Exec(ctx, `UPDATE users SET memory_cleared_at=now() WHERE id=$1`, user); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM memories WHERE user_id=$1`, user); err != nil {
		return err
	}
	if err = audit(ctx, tx, user, "all_memories_deleted", user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) DeleteCompanionMemories(ctx context.Context, user, companion string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	var owner string
	if err = tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&owner); err != nil {
		return dbError(err)
	}
	tag, err := tx.Exec(ctx, `UPDATE companions SET memory_cleared_at=now() WHERE owner_id=$1 AND id=$2`, user, companion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `DELETE FROM memories WHERE user_id=$1 AND companion_id=$2`, user, companion); err != nil {
		return err
	}
	if err = audit(ctx, tx, user, "companion_memories_deleted", companion); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) RelevantMemories(ctx context.Context, user, query string) ([]domain.Memory, error) {
	persona, err := p.GetPersona(ctx, user)
	if err != nil {
		return nil, err
	}
	return p.RelevantCompanionMemories(ctx, user, persona.ID, query)
}
func (p *Postgres) CompanionMemories(ctx context.Context, user, companion string) ([]domain.Memory, error) {
	if _, err := p.GetCompanion(ctx, user, companion); err != nil {
		return nil, err
	}
	rows, err := p.Pool.Query(ctx, `SELECT `+memoryColumns+` FROM memories WHERE user_id=$1 AND companion_id=$2 ORDER BY created_at DESC LIMIT 500`, user, companion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Memory{}
	for rows.Next() {
		m, scanErr := scanMemory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (p *Postgres) RelevantCompanionMemories(ctx context.Context, user, companion, query string) ([]domain.Memory, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+memoryColumns+` FROM memories WHERE user_id=$1 AND companion_id=$3 AND consent_status='approved' AND confidence>=0.8 AND (expires_at IS NULL OR expires_at>now()) ORDER BY CASE WHEN type='boundary' THEN 0 ELSE 1 END, embedding <=> $2::vector LIMIT 50`, user, lexicalVector(query), companion)
	if err != nil {
		return nil, fmt.Errorf("memory retrieval: %w", err)
	}
	defer rows.Close()
	result := []domain.Memory{}
	for rows.Next() {
		m, scanErr := scanMemory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
