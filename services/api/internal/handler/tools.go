package handler

import (
	"companion/api/internal/domain"
	"context"
	"net/http"
	"time"
)

func (a *API) tools(w http.ResponseWriter, r *http.Request) {
	if a.Tools == nil {
		fail(w, r, domain.ErrUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	result, err := a.Tools.List(ctx, userID(r), r.URL.Query().Get("companion_id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) invokeTool(w http.ResponseWriter, r *http.Request) {
	if a.Tools == nil {
		fail(w, r, domain.ErrUnavailable)
		return
	}
	var body struct {
		CompanionID string         `json:"companion_id"`
		ToolID      string         `json:"tool_id"`
		Arguments   map[string]any `json:"arguments"`
	}
	if err := decode(w, r, &body); err != nil || body.CompanionID == "" || body.ToolID == "" || body.Arguments == nil {
		fail(w, r, domain.ErrInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	result, err := a.Tools.Invoke(ctx, userID(r), body.CompanionID, body.ToolID, requestID(r), body.Arguments)
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
