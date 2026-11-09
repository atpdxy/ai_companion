package infrastructure

import (
	"companion/api/internal/domain"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

func seedCompanions(ctx context.Context, tx pgx.Tx, user string) error {
	for _, card := range domain.PresetCards() {
		id := "card_" + user + "_" + card.Definition.Relationship
		raw, err := json.Marshal(card.Definition)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO character_cards(id,owner_id,definition) VALUES($1,$2,$3)`, id, user, raw); err != nil {
			return err
		}
		if card.Definition.Relationship == "lover" {
			companion := "cmp_" + user + "_default"
			if _, err = tx.Exec(ctx, `INSERT INTO companions(id,owner_id,card_id,config) VALUES($1,$2,$3,$4)`, companion, user, id, raw); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE users SET active_companion_id=$2 WHERE id=$1`, user, companion); err != nil {
				return err
			}
		}
	}
	return nil
}
func scanCompanion(row pgx.Row) (domain.Persona, error) {
	var p domain.Persona
	var raw []byte
	var id, card string
	var version int
	if err := row.Scan(&id, &card, &raw, &version); err != nil {
		return p, dbError(err)
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	p.ID = id
	p.CardID = card
	p.Version = version
	return p, nil
}
func (p *Postgres) GetCompanion(ctx context.Context, user, id string) (domain.Persona, error) {
	return scanCompanion(p.Pool.QueryRow(ctx, `SELECT id,card_id,config,version FROM companions WHERE owner_id=$1 AND id=$2`, user, id))
}
func (p *Postgres) ListCompanions(ctx context.Context, user string) ([]domain.Persona, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,card_id,config,version FROM companions WHERE owner_id=$1 ORDER BY created_at,id`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Persona{}
	for rows.Next() {
		item, scanErr := scanCompanion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (p *Postgres) ListCharacterCards(ctx context.Context, user string) ([]domain.CharacterCard, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,definition,version FROM character_cards WHERE owner_id=$1 ORDER BY created_at,id`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.CharacterCard{}
	for rows.Next() {
		card := domain.CharacterCard{SchemaVersion: domain.CardSchemaVersion}
		var raw []byte
		if err = rows.Scan(&card.ID, &raw, &card.Version); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &card.Definition); err != nil {
			return nil, err
		}
		result = append(result, card)
	}
	return result, rows.Err()
}
func (p *Postgres) CreateCharacterCard(ctx context.Context, user string, card domain.CharacterCard) (domain.CharacterCard, error) {
	card.ID = newID("card_")
	card.Version = 1
	card.Definition.ID = ""
	card.Definition.CardID = ""
	card.Definition.Version = 1
	raw, err := json.Marshal(card.Definition)
	if err != nil {
		return card, err
	}
	_, err = p.Pool.Exec(ctx, `INSERT INTO character_cards(id,owner_id,definition) VALUES($1,$2,$3)`, card.ID, user, raw)
	return card, err
}
func (p *Postgres) CreateCompanion(ctx context.Context, user, cardID, name string) (domain.Persona, error) {
	var definition []byte
	var result domain.Persona
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer rollback(ctx, tx)
	if err = tx.QueryRow(ctx, `SELECT definition FROM character_cards WHERE id=$1 AND owner_id=$2`, cardID, user).Scan(&definition); err != nil {
		return result, dbError(err)
	}
	if err = json.Unmarshal(definition, &result); err != nil {
		return result, err
	}
	result.Name = name
	result.ID = ""
	result.CardID = ""
	result.Version = 1
	if !result.ValidCharacter() {
		return result, domain.ErrInvalid
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	result.ID = newID("cmp_")
	result.CardID = cardID
	if _, err = tx.Exec(ctx, `INSERT INTO companions(id,owner_id,card_id,config) VALUES($1,$2,$3,$4)`, result.ID, user, cardID, raw); err != nil {
		return result, err
	}
	if err = audit(ctx, tx, user, "companion_created", result.ID); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
func (p *Postgres) UpdateCompanion(ctx context.Context, user, id string, persona domain.Persona) (domain.Persona, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return persona, err
	}
	defer rollback(ctx, tx)
	if err = tx.QueryRow(ctx, `UPDATE companions SET version=version+1 WHERE id=$1 AND owner_id=$2 RETURNING card_id,version`, id, user).Scan(&persona.CardID, &persona.Version); err != nil {
		return persona, dbError(err)
	}
	persona.ID = id
	raw, err := json.Marshal(persona)
	if err != nil {
		return persona, err
	}
	if _, err = tx.Exec(ctx, `UPDATE companions SET config=$3 WHERE id=$1 AND owner_id=$2`, id, user, raw); err != nil {
		return persona, err
	}
	if err = audit(ctx, tx, user, "companion_updated", id); err != nil {
		return persona, err
	}
	return persona, tx.Commit(ctx)
}
func (p *Postgres) ActivateCompanion(ctx context.Context, user, id string) (domain.Persona, error) {
	persona, err := p.GetCompanion(ctx, user, id)
	if err != nil {
		return persona, err
	}
	_, err = p.Pool.Exec(ctx, `UPDATE users SET active_companion_id=$2 WHERE id=$1`, user, id)
	return persona, err
}
func (p *Postgres) ConversationPersona(ctx context.Context, user, conversation string) (domain.Persona, error) {
	return scanCompanion(p.Pool.QueryRow(ctx, `SELECT a.id,a.card_id,a.config,a.version FROM companions a JOIN conversations c ON c.companion_id=a.id AND c.user_id=a.owner_id WHERE c.id=$1 AND c.user_id=$2`, conversation, user))
}
func (p *Postgres) CreateCompanionConversation(ctx context.Context, user, companion, title string) (domain.Conversation, error) {
	item := domain.Conversation{ID: newID("conv_"), Title: title, CompanionID: companion}
	err := p.Pool.QueryRow(ctx, `INSERT INTO conversations(id,user_id,title,companion_id) SELECT $1,$2,$3,id FROM companions WHERE owner_id=$2 AND id=$4 RETURNING created_at`, item.ID, user, title, companion).Scan(&item.CreatedAt)
	return item, dbError(err)
}
