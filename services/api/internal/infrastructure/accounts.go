package infrastructure

import (
	"context"
	"encoding/json"

	"companion/api/internal/domain"
)

func (p *Postgres) CreateUser(ctx context.Context, email, hash string) (domain.User, error) {
	u := domain.User{ID: newID("usr_"), Email: email, PasswordHash: hash}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return u, err
	}
	defer rollback(ctx, tx)
	err = tx.QueryRow(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,$3) RETURNING created_at`, u.ID, email, hash).Scan(&u.CreatedAt)
	if err != nil {
		return u, dbError(err)
	}
	config, err := json.Marshal(domain.DefaultPersona())
	if err != nil {
		return u, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO personas(owner_id,config) VALUES($1,$2)`, u.ID, config); err != nil {
		return u, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO notification_prefs(user_id) VALUES($1)`, u.ID); err != nil {
		return u, err
	}
	if err = seedCompanions(ctx, tx, u.ID); err != nil {
		return u, err
	}
	return u, tx.Commit(ctx)
}
func (p *Postgres) FindUser(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := p.Pool.QueryRow(ctx, `SELECT id,email,password_hash,memory_consent,created_at FROM users WHERE email=$1`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.MemoryConsent, &u.CreatedAt)
	return u, dbError(err)
}
func (p *Postgres) GetUser(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := p.Pool.QueryRow(ctx, `SELECT id,email,memory_consent,created_at FROM users WHERE id=$1`, id).Scan(&u.ID, &u.Email, &u.MemoryConsent, &u.CreatedAt)
	return u, dbError(err)
}
func (p *Postgres) SetMemoryConsent(ctx context.Context, id string, consent bool) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `UPDATE users SET memory_consent=$2 WHERE id=$1`, id, consent)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if !consent {
		if _, err = tx.Exec(ctx, `UPDATE users SET memory_cleared_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM memories WHERE user_id=$1 AND consent_status='pending'`, id); err != nil {
			return err
		}
	}
	if err = audit(ctx, tx, id, "memory_consent_changed", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) GetPersona(ctx context.Context, user string) (domain.Persona, error) {
	return scanCompanion(p.Pool.QueryRow(ctx, `SELECT a.id,a.card_id,a.config,a.version FROM companions a JOIN users u ON u.active_companion_id=a.id AND u.id=a.owner_id WHERE u.id=$1`, user))
}
func (p *Postgres) UpdatePersona(ctx context.Context, user string, update domain.Persona) (domain.Persona, error) {
	current, err := p.GetPersona(ctx, user)
	if err != nil {
		return current, err
	}
	current.Name = update.Name
	current.UserName = update.UserName
	current.Warmth = update.Warmth
	current.Brevity = update.Brevity
	current.Humor = update.Humor
	return p.UpdateCompanion(ctx, user, current.ID, current)
}
func (p *Postgres) DeleteAccount(ctx context.Context, id string) error {
	tag, err := p.Pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func (p *Postgres) Export(ctx context.Context, id string) (domain.AccountExport, error) {
	var result domain.AccountExport
	var err error
	if result.User, err = p.GetUser(ctx, id); err != nil {
		return result, err
	}
	if result.Persona, err = p.GetPersona(ctx, id); err != nil {
		return result, err
	}
	if result.Conversations, err = p.listConversations(ctx, id, 0); err != nil {
		return result, err
	}
	if result.Memories, err = p.listMemories(ctx, id, 0); err != nil {
		return result, err
	}
	if result.NotificationPreferences, err = p.GetNotificationPreferences(ctx, id); err != nil {
		return result, err
	}
	if result.Events, err = p.listEvents(ctx, id, 0); err != nil {
		return result, err
	}
	if result.Notifications, err = p.listNotifications(ctx, id, 0, true); err != nil {
		return result, err
	}
	if result.Companions, err = p.ListCompanions(ctx, id); err != nil {
		return result, err
	}
	if result.CharacterCards, err = p.ListCharacterCards(ctx, id); err != nil {
		return result, err
	}
	result.Messages = []domain.Message{}
	for _, conversation := range result.Conversations {
		messages, fetchErr := p.Messages(ctx, id, conversation.ID)
		if fetchErr != nil {
			return result, fetchErr
		}
		result.Messages = append(result.Messages, messages...)
	}
	return result, nil
}
