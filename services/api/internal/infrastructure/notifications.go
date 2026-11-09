package infrastructure

import (
	"context"
	"time"

	"companion/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

func scanPreferences(row pgx.Row) (domain.NotificationPreferences, error) {
	var p domain.NotificationPreferences
	err := row.Scan(&p.OptIn, &p.NoContact, &p.QuietStart, &p.QuietEnd, &p.DailyLimit, &p.Timezone)
	return p, dbError(err)
}
func (p *Postgres) GetNotificationPreferences(ctx context.Context, user string) (domain.NotificationPreferences, error) {
	return scanPreferences(p.Pool.QueryRow(ctx, `SELECT opt_in,no_contact,quiet_start,quiet_end,daily_limit,timezone FROM notification_prefs WHERE user_id=$1`, user))
}
func (p *Postgres) SetNotificationPreferences(ctx context.Context, user string, prefs domain.NotificationPreferences) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	if _, err = tx.Exec(ctx, `UPDATE notification_prefs SET opt_in=$2,no_contact=$3,quiet_start=$4,quiet_end=$5,daily_limit=$6,timezone=$7 WHERE user_id=$1`, user, prefs.OptIn, prefs.NoContact, prefs.QuietStart, prefs.QuietEnd, prefs.DailyLimit, prefs.Timezone); err != nil {
		return err
	}
	if err = audit(ctx, tx, user, "notification_preferences_changed", user); err != nil {
		return err
	}
	if prefs.OptIn && !prefs.NoContact {
		if _, err = tx.Exec(ctx, `UPDATE scheduled_events SET next_check_at='-infinity'::timestamptz WHERE user_id=$1 AND status='pending'`, user); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (p *Postgres) PauseNotifications(ctx context.Context, user string) error {
	_, err := p.Pool.Exec(ctx, `UPDATE notification_prefs SET opt_in=false,no_contact=true WHERE user_id=$1`, user)
	return err
}
func scanEvent(row pgx.Row) (domain.ScheduledEvent, error) {
	var e domain.ScheduledEvent
	err := row.Scan(&e.ID, &e.Content, &e.DueAt, &e.ExpiresAt, &e.Recurrence, &e.Status, &e.ClientKey)
	return e, dbError(err)
}

const eventColumns = `id,content,due_at,expires_at,recurrence,status,client_key`

func (p *Postgres) ListEvents(ctx context.Context, user string) ([]domain.ScheduledEvent, error) {
	return p.listEvents(ctx, user, 200)
}
func (p *Postgres) listEvents(ctx context.Context, user string, limit int) ([]domain.ScheduledEvent, error) {
	rows, err := p.Pool.Query(ctx, `SELECT `+eventColumns+` FROM scheduled_events WHERE user_id=$1 ORDER BY due_at DESC LIMIT NULLIF($2,0)`, user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ScheduledEvent{}
	for rows.Next() {
		e, scanErr := scanEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func (p *Postgres) CreateEvent(ctx context.Context, user string, event domain.ScheduledEvent) (domain.ScheduledEvent, error) {
	event.DueAt = event.DueAt.Truncate(time.Microsecond)
	event.ExpiresAt = event.ExpiresAt.Truncate(time.Microsecond)
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return event, err
	}
	defer rollback(ctx, tx)
	var optedIn, noContact bool
	if err = tx.QueryRow(ctx, `SELECT opt_in,no_contact FROM notification_prefs WHERE user_id=$1 FOR UPDATE`, user).Scan(&optedIn, &noContact); err != nil {
		return event, dbError(err)
	}
	if !optedIn || noContact {
		return event, domain.ErrInvalid
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM scheduled_events WHERE user_id=$1 AND status='pending'`, user).Scan(&count); err != nil {
		return event, err
	}
	if count >= 100 {
		return event, domain.ErrConflict
	}
	created, err := scanEvent(tx.QueryRow(ctx, `INSERT INTO scheduled_events(id,user_id,content,due_at,expires_at,recurrence,client_key,original_due_at,next_check_at) VALUES($1,$2,$3,$4,$5,$6,$7,$4,'-infinity'::timestamptz) ON CONFLICT(user_id,client_key) DO UPDATE SET client_key=EXCLUDED.client_key RETURNING `+eventColumns, newID("evt_"), user, event.Content, event.DueAt, event.ExpiresAt, event.Recurrence, event.ClientKey))
	if err != nil {
		return event, err
	}
	var original time.Time
	if err = tx.QueryRow(ctx, `SELECT original_due_at FROM scheduled_events WHERE id=$1 AND user_id=$2`, created.ID, user).Scan(&original); err != nil {
		return event, err
	}
	if !original.Equal(event.DueAt) || created.Content != event.Content || created.Recurrence != event.Recurrence || !created.ExpiresAt.Equal(event.ExpiresAt) {
		return event, domain.ErrConflict
	}
	if err = audit(ctx, tx, user, "event_created", created.ID); err != nil {
		return event, err
	}
	return created, tx.Commit(ctx)
}
func (p *Postgres) CancelEvent(ctx context.Context, user, id string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `UPDATE scheduled_events SET status='cancelled' WHERE id=$1 AND user_id=$2`, id, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err = audit(ctx, tx, user, "event_cancelled", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) ListNotifications(ctx context.Context, user string) ([]domain.Notification, error) {
	return p.listNotifications(ctx, user, 100, false)
}
func (p *Postgres) listNotifications(ctx context.Context, user string, limit int, export bool) ([]domain.Notification, error) {
	rows, err := p.Pool.Query(ctx, `SELECT n.id,n.event_id,n.content,n.delivered_at FROM notifications n JOIN notification_prefs p ON p.user_id=n.user_id WHERE n.user_id=$1 AND ($3 OR (p.opt_in AND NOT p.no_contact)) ORDER BY n.delivered_at DESC LIMIT NULLIF($2,0)`, user, limit, export)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Notification{}
	for rows.Next() {
		var n domain.Notification
		if err = rows.Scan(&n.ID, &n.EventID, &n.Content, &n.DeliveredAt); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

// RunScheduler owns one cancellable loop. SQL locks serialize multiple API replicas.
func (p *Postgres) RunScheduler(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			run, cancel := context.WithTimeout(ctx, 8*time.Second)
			if err := p.DeliverDue(run, now); err != nil {
				onError(err)
			}
			cancel()
		}
	}
}
func (p *Postgres) DeliverDue(ctx context.Context, now time.Time) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	if _, err = tx.Exec(ctx, `UPDATE scheduled_events SET status='expired' WHERE status='pending' AND expires_at<=$1`, now); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT `+eventColumns+`,user_id FROM scheduled_events WHERE status='pending' AND due_at<=$1 AND next_check_at<=$1 ORDER BY next_check_at,due_at LIMIT 50 FOR UPDATE SKIP LOCKED`, now)
	if err != nil {
		return err
	}
	type ownedEvent struct {
		event domain.ScheduledEvent
		user  string
	}
	events := []ownedEvent{}
	for rows.Next() {
		var owned ownedEvent
		e := &owned.event
		if err = rows.Scan(&e.ID, &e.Content, &e.DueAt, &e.ExpiresAt, &e.Recurrence, &e.Status, &e.ClientKey, &owned.user); err != nil {
			rows.Close()
			return err
		}
		events = append(events, owned)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, owned := range events {
		e := owned.event
		if !e.ExpiresAt.After(now) {
			if _, err = tx.Exec(ctx, `UPDATE scheduled_events SET status='expired' WHERE id=$1 AND user_id=$2`, e.ID, owned.user); err != nil {
				return err
			}
			continue
		}
		prefs, prefErr := scanPreferences(tx.QueryRow(ctx, `SELECT opt_in,no_contact,quiet_start,quiet_end,daily_limit,timezone FROM notification_prefs WHERE user_id=$1 FOR UPDATE`, owned.user))
		if prefErr != nil {
			return prefErr
		}
		location, locationErr := time.LoadLocation(prefs.Timezone)
		if locationErr != nil {
			return locationErr
		}
		local := now.In(location)
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND delivered_at>=$2`, owned.user, start).Scan(&count); err != nil {
			return err
		}
		if !prefs.Eligible(now, count) {
			if _, err = tx.Exec(ctx, `UPDATE scheduled_events SET next_check_at=$3 WHERE id=$1 AND user_id=$2`, e.ID, owned.user, now.Add(time.Minute)); err != nil {
				return err
			}
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO notifications(id,user_id,event_id,occurrence_at,content,delivered_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(event_id,occurrence_at) DO NOTHING`, newID("ntf_"), owned.user, e.ID, e.DueAt, "你约定的提醒："+e.Content, now); err != nil {
			return err
		}
		if e.Recurrence == "none" {
			_, err = tx.Exec(ctx, `UPDATE scheduled_events SET status='delivered' WHERE id=$1 AND user_id=$2`, e.ID, owned.user)
		} else {
			days := 1
			if e.Recurrence == "weekly" {
				days = 7
			}
			next := e.DueAt.In(location)
			for !next.After(now) {
				next = next.AddDate(0, 0, days)
			}
			_, err = tx.Exec(ctx, `UPDATE scheduled_events SET due_at=$3 WHERE id=$1 AND user_id=$2`, e.ID, owned.user, next)
		}
		if err != nil {
			return err
		}
		if err = audit(ctx, tx, owned.user, "notification_delivered", e.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
