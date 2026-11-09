package handler

import (
	"companion/api/internal/domain"
	"net/http"
	"strings"
)

func (a *API) companions(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.ListCompanions(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) companion(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.GetCompanion(r.Context(), userID(r), r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) newCompanion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardID string `json:"card_id"`
		Name   string `json:"name"`
	}
	if err := decode(w, r, &body); err != nil || len([]rune(body.Name)) > 24 || strings.TrimSpace(body.Name) == "" {
		fail(w, r, domain.ErrInvalid)
		return
	}
	result, err := a.Store.CreateCompanion(r.Context(), userID(r), body.CardID, body.Name)
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 201, result)
}
func (a *API) putCompanion(w http.ResponseWriter, r *http.Request) {
	var body domain.Persona
	if err := decode(w, r, &body); err != nil || !body.ValidCharacter() {
		fail(w, r, domain.ErrInvalid)
		return
	}
	result, err := a.Store.UpdateCompanion(r.Context(), userID(r), r.PathValue("id"), body)
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) activateCompanion(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.ActivateCompanion(r.Context(), userID(r), r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) cards(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.ListCharacterCards(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) newCard(w http.ResponseWriter, r *http.Request) {
	var body domain.CharacterCard
	if err := decode(w, r, &body); err != nil || body.SchemaVersion != domain.CardSchemaVersion || !body.Definition.ValidCharacter() {
		fail(w, r, domain.ErrInvalid)
		return
	}
	result, err := a.Store.CreateCharacterCard(r.Context(), userID(r), body)
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 201, result)
}
