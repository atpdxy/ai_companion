package handler

import (
	"net/http"
	"strings"
	"time"

	"companion/api/internal/domain"
)

func (a *API) getNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.GetNotificationPreferences(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) putNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	var body domain.NotificationPreferences
	if err := decode(w, r, &body); err != nil || !body.Valid() {
		fail(w, r, domain.ErrInvalid)
		return
	}
	if err := a.Store.SetNotificationPreferences(r.Context(), userID(r), body); err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, body)
}
func (a *API) events(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.ListEvents(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) newEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content    string    `json:"content"`
		DueAt      time.Time `json:"due_at"`
		ExpiresAt  time.Time `json:"expires_at"`
		Recurrence string    `json:"recurrence"`
		ClientKey  string    `json:"client_key"`
	}
	if err := decode(w, r, &body); err != nil || len([]rune(body.Content)) > 500 || strings.TrimSpace(body.Content) == "" || !body.DueAt.After(time.Now()) || body.DueAt.After(time.Now().AddDate(1, 0, 0)) || (body.Recurrence != "none" && body.Recurrence != "daily" && body.Recurrence != "weekly") || len(body.ClientKey) < 8 || len(body.ClientKey) > 80 {
		fail(w, r, domain.ErrInvalid)
		return
	}
	if body.ExpiresAt.IsZero() {
		body.ExpiresAt = body.DueAt.AddDate(0, 0, 7)
	}
	if !body.ExpiresAt.After(body.DueAt) || body.ExpiresAt.After(time.Now().AddDate(2, 0, 0)) {
		fail(w, r, domain.ErrInvalid)
		return
	}
	result, err := a.Store.CreateEvent(r.Context(), userID(r), domain.ScheduledEvent{Content: body.Content, DueAt: body.DueAt, ExpiresAt: body.ExpiresAt, Recurrence: body.Recurrence, ClientKey: body.ClientKey})
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 201, result)
}
func (a *API) cancelEvent(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.CancelEvent(r.Context(), userID(r), r.PathValue("id")); err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, nil)
}
func (a *API) notifications(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.ListNotifications(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
