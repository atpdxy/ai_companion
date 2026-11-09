package handler

import (
	"companion/api/internal/domain"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

func (a *API) transcribe(w http.ResponseWriter, r *http.Request) {
	if !a.ASREnabled || a.Voice == nil {
		fail(w, r, domain.ErrUnavailable)
		return
	}
	var body struct {
		CompanionID string `json:"companion_id"`
		AudioBase64 string `json:"audio_base64"`
		Format      string `json:"format"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 14_100_000)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		fail(w, r, domain.ErrInvalid)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		fail(w, r, domain.ErrInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 95*time.Second)
	defer cancel()
	result, err := a.Voice.Transcribe(ctx, userID(r), body.CompanionID, domain.TranscriptionRequest{AudioBase64: body.AudioBase64, Format: body.Format, RequestID: requestID(r)})
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) speech(w http.ResponseWriter, r *http.Request) {
	if !a.TTSEnabled || a.Voice == nil {
		fail(w, r, domain.ErrUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 95*time.Second)
	defer cancel()
	audio, err := a.Voice.Speech(ctx, userID(r), r.PathValue("id"), r.PathValue("message"), requestID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	if _, err = w.Write(audio); err != nil {
		slog.Info("audio_disconnected", "trace_id", requestID(r))
	}
}
