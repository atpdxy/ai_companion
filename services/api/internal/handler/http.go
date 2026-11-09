package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"companion/api/internal/domain"
	"companion/api/internal/repository"
	"companion/api/internal/service"
)

type contextKey string

const userKey contextKey = "user"
const requestKey contextKey = "request"

type API struct {
	Tools         *service.Tools
	Voice         *service.Voice
	ASREnabled    bool
	TTSEnabled    bool
	Mode          string
	Store         repository.Store
	Auth          *service.Auth
	Chat          *service.Chat
	Limiter       repository.Limiter
	Origin        string
	SecureCookies bool
	Ready         func(context.Context) error
}
type envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func requestID(r *http.Request) string {
	value, _ := r.Context().Value(requestKey).(string)
	return value
}
func userID(r *http.Request) string { value, _ := r.Context().Value(userKey).(string); return value }
func write(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{"OK", "success", data, requestID(r)}); err != nil {
		slog.Warn("response_write_failed", "trace_id", requestID(r))
	}
}
func fail(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "INTERNAL_ERROR", "服务暂时无法完成请求"
	switch {
	case errors.Is(err, domain.ErrInvalid):
		status, code, message = 400, "INVALID_INPUT", "请检查输入内容"
	case errors.Is(err, domain.ErrUnauthorized):
		status, code, message = 401, "UNAUTHORIZED", "请登录或检查账号密码"
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = 404, "NOT_FOUND", "内容不存在或无权访问"
	case errors.Is(err, domain.ErrConflict):
		status, code, message = 409, "CONFLICT", "内容已存在或会话正在回复，请稍后重试"
	case errors.Is(err, domain.ErrUnavailable):
		status, code, message = 503, "UNAVAILABLE", "上游服务暂时不可用"
	}
	if status >= 500 {
		slog.Error("request_failed", "trace_id", requestID(r), "code", code)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(envelope{code, message, nil, requestID(r)}); encodeErr != nil {
		slog.Warn("response_write_failed", "trace_id", requestID(r))
	}
}
func decode(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 40000)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return domain.ErrInvalid
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.ErrInvalid
	}
	return nil
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, r, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if a.Ready != nil {
			if err := a.Ready(ctx); err != nil {
				fail(w, r, domain.ErrUnavailable)
				return
			}
		}
		write(w, r, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/logout", a.authenticated(a.logout))
	mux.HandleFunc("GET /api/v1/me", a.authenticated(a.me))
	mux.HandleFunc("GET /api/v1/status", a.authenticated(func(w http.ResponseWriter, r *http.Request) {
		mode := a.Mode
		if mode == "" {
			mode = "demo"
		}
		write(w, r, 200, map[string]any{"mode": mode, "version": "0.2.0", "asr_enabled": a.ASREnabled, "tts_enabled": a.TTSEnabled})
	}))
	mux.HandleFunc("PATCH /api/v1/me/preferences", a.authenticated(a.preferences))
	mux.HandleFunc("GET /api/v1/me/export", a.authenticated(a.export))
	mux.HandleFunc("DELETE /api/v1/me", a.authenticated(a.deleteAccount))
	mux.HandleFunc("GET /api/v1/persona", a.authenticated(a.getPersona))
	mux.HandleFunc("PUT /api/v1/persona", a.authenticated(a.putPersona))
	mux.HandleFunc("GET /api/v1/conversations", a.authenticated(a.conversations))
	mux.HandleFunc("POST /api/v1/conversations", a.authenticated(a.newConversation))
	mux.HandleFunc("GET /api/v1/conversations/{id}/messages", a.authenticated(a.messages))
	mux.HandleFunc("POST /api/v1/conversations/{id}/messages", a.authenticated(a.send))
	mux.HandleFunc("GET /api/v1/conversations/{id}/stream", a.authenticated(a.replay))
	mux.HandleFunc("GET /api/v1/memories", a.authenticated(a.memories))
	mux.HandleFunc("PATCH /api/v1/memories/{id}", a.authenticated(a.updateMemory))
	mux.HandleFunc("DELETE /api/v1/memories/{id}", a.authenticated(a.deleteMemory))
	mux.HandleFunc("DELETE /api/v1/memories", a.authenticated(a.deleteMemories))
	mux.HandleFunc("POST /api/v1/feedback", a.authenticated(a.feedback))
	mux.HandleFunc("GET /api/v1/integrations/tools", a.authenticated(a.tools))
	mux.HandleFunc("POST /api/v1/integrations/tools/invoke", a.authenticated(a.invokeTool))
	mux.HandleFunc("POST /api/v1/audio/transcriptions", a.authenticated(a.transcribe))
	mux.HandleFunc("POST /api/v1/conversations/{id}/messages/{message}/audio", a.authenticated(a.speech))
	mux.HandleFunc("GET /api/v1/companions", a.authenticated(a.companions))
	mux.HandleFunc("POST /api/v1/companions", a.authenticated(a.newCompanion))
	mux.HandleFunc("GET /api/v1/companions/{id}", a.authenticated(a.companion))
	mux.HandleFunc("PUT /api/v1/companions/{id}", a.authenticated(a.putCompanion))
	mux.HandleFunc("POST /api/v1/companions/{id}/activate", a.authenticated(a.activateCompanion))
	mux.HandleFunc("GET /api/v1/character-cards", a.authenticated(a.cards))
	mux.HandleFunc("POST /api/v1/character-cards", a.authenticated(a.newCard))
	mux.HandleFunc("GET /api/v1/notification-preferences", a.authenticated(a.getNotificationPreferences))
	mux.HandleFunc("PUT /api/v1/notification-preferences", a.authenticated(a.putNotificationPreferences))
	mux.HandleFunc("GET /api/v1/events", a.authenticated(a.events))
	mux.HandleFunc("POST /api/v1/events", a.authenticated(a.newEvent))
	mux.HandleFunc("DELETE /api/v1/events/{id}", a.authenticated(a.cancelEvent))
	mux.HandleFunc("GET /api/v1/notifications", a.authenticated(a.notifications))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		trace := "req_" + hex.EncodeToString(id[:])
		r = r.WithContext(context.WithValue(r.Context(), requestKey, trace))
		w.Header().Set("X-Request-ID", trace)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("request_panic", "trace_id", trace)
				fail(w, r, errors.New("panic"))
			}
			slog.Info("http_request", "trace_id", trace, "method", r.Method, "latency_ms", time.Since(started).Milliseconds())
		}()
		// Same-origin deployment through Vite/nginx; no permissive CORS.
		if origin := r.Header.Get("Origin"); origin != "" && origin != a.Origin {
			fail(w, r, domain.ErrUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *API) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			cookie, err := r.Cookie("companion_session")
			if err != nil {
				fail(w, r, domain.ErrUnauthorized)
				return
			}
			token = cookie.Value
			if r.Method != "GET" && r.Header.Get("Origin") != a.Origin {
				fail(w, r, domain.ErrUnauthorized)
				return
			}
		}
		id, err := a.Auth.Verify(token)
		if err != nil {
			fail(w, r, err)
			return
		}
		if _, err = a.Store.GetUser(r.Context(), id); err != nil {
			fail(w, r, domain.ErrUnauthorized)
			return
		}
		allowed, err := a.Limiter.Allow(r.Context(), id, 180)
		if err != nil {
			fail(w, r, domain.ErrUnavailable)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(429)
			if err = json.NewEncoder(w).Encode(envelope{"RATE_LIMITED", "请稍后重试", nil, requestID(r)}); err != nil {
				slog.Warn("response_write_failed")
			}
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, id)))
	}
}

func (a *API) cookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: "companion_session", Value: token, Path: "/", HttpOnly: true, Secure: a.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: maxAge})
}
func (a *API) authRate(w http.ResponseWriter, r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	allowed, err := a.Limiter.Allow(r.Context(), "auth:"+ip, 10)
	if err != nil {
		fail(w, r, domain.ErrUnavailable)
		return false
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		if err = json.NewEncoder(w).Encode(envelope{"RATE_LIMITED", "登录尝试过多，请稍后再试", nil, requestID(r)}); err != nil {
			slog.Warn("response_write_failed")
		}
		return false
	}
	return true
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Adult    bool   `json:"adult_confirmed"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if !a.authRate(w, r) {
		return
	}
	var body credentials
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	u, token, err := a.Auth.Register(r.Context(), body.Email, body.Password, body.Adult)
	if err != nil {
		fail(w, r, err)
		return
	}
	a.cookie(w, token, 43200)
	write(w, r, 201, u)
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.authRate(w, r) {
		return
	}
	var body credentials
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	u, token, err := a.Auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		fail(w, r, err)
		return
	}
	a.cookie(w, token, 43200)
	write(w, r, 200, u)
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	a.cookie(w, "", -1)
	write(w, r, 200, nil)
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.Store.GetUser(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, u)
}
func (a *API) preferences(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Consent *bool `json:"memory_consent"`
	}
	if err := decode(w, r, &body); err != nil || body.Consent == nil {
		fail(w, r, domain.ErrInvalid)
		return
	}
	if err := a.Store.SetMemoryConsent(r.Context(), userID(r), *body.Consent); err != nil {
		fail(w, r, err)
		return
	}
	a.me(w, r)
}
func (a *API) export(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.Export(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="companion-export.json"`)
	write(w, r, 200, result)
}
func (a *API) deleteAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Confirmation string `json:"confirmation"`
	}
	if err := decode(w, r, &body); err != nil || body.Confirmation != "DELETE" {
		fail(w, r, domain.ErrInvalid)
		return
	}
	if err := a.Store.DeleteAccount(r.Context(), userID(r)); err != nil {
		fail(w, r, err)
		return
	}
	a.cookie(w, "", -1)
	write(w, r, 200, nil)
}
func (a *API) getPersona(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.GetPersona(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) putPersona(w http.ResponseWriter, r *http.Request) {
	var body domain.Persona
	if err := decode(w, r, &body); err != nil || !body.Valid() {
		fail(w, r, domain.ErrInvalid)
		return
	}
	result, err := a.Store.UpdatePersona(r.Context(), userID(r), body)
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) conversations(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.ListConversations(r.Context(), userID(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) newConversation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CompanionID string `json:"companion_id"`
		Title       string `json:"title"`
	}
	if err := decode(w, r, &body); err != nil || len([]rune(body.Title)) > 80 {
		fail(w, r, domain.ErrInvalid)
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		body.Title = "新的心事"
	}
	var result domain.Conversation
	var err error
	if body.CompanionID == "" {
		result, err = a.Store.CreateConversation(r.Context(), userID(r), body.Title)
	} else {
		result, err = a.Store.CreateCompanionConversation(r.Context(), userID(r), body.CompanionID, body.Title)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 201, result)
}
func (a *API) messages(w http.ResponseWriter, r *http.Request) {
	result, err := a.Store.Messages(r.Context(), userID(r), r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}

type sseWriter struct {
	w          http.ResponseWriter
	controller *http.ResponseController
	sequence   int
}

func (s *sseWriter) emit(event domain.Event) error {
	switch event.Name {
	case "message_start":
		s.sequence = 1
	case "delta":
		s.sequence = 2
	case "message_end":
		s.sequence = 3
	default:
		s.sequence = 4
	}
	if err := s.controller.SetWriteDeadline(time.Now().Add(150 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "id: %d\nevent: %s\ndata: %s\n\n", s.sequence, event.Name, event.Data); err != nil {
		return err
	}
	return s.controller.Flush()
}
func streamWriter(w http.ResponseWriter) *sseWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	return &sseWriter{w: w, controller: http.NewResponseController(w)}
}
func (a *API) send(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
		TurnID  string `json:"turn_id"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 145*time.Second)
	defer cancel()
	turn, err := a.Chat.Prepare(ctx, userID(r), r.PathValue("id"), body.TurnID, body.Content)
	if err != nil {
		fail(w, r, err)
		return
	}
	stream := strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	if turn.Status == "completed" {
		if stream {
			replayResult(streamWriter(w), *turn.Result, 0)
		} else {
			write(w, r, 200, turn.Result)
		}
		return
	}
	var emit func(domain.Event) error
	var writer *sseWriter
	if stream {
		writer = streamWriter(w)
		emit = writer.emit
	} else {
		emit = func(domain.Event) error { return nil }
	}
	result, err := a.Chat.Respond(ctx, userID(r), r.PathValue("id"), body.TurnID, requestID(r), body.Content, emit)
	if err != nil {
		if stream {
			if emitErr := writer.emit(domain.Event{Name: "error", Data: json.RawMessage(`{"code":"TURN_INCOMPLETE","message":"回复未完成，可重试此轮。"}`)}); emitErr != nil {
				slog.Info("stream_disconnected", "trace_id", requestID(r))
			}
		} else {
			fail(w, r, err)
		}
		return
	}
	if !stream {
		write(w, r, 200, result)
	}
}
func replayResult(writer *sseWriter, result domain.AgentResult, after int) {
	result.Emotion = nil
	result.Candidates = nil
	data, err := json.Marshal(result)
	if err != nil {
		slog.Error("replay_encode_failed")
		return
	}
	text, err := json.Marshal(map[string]string{"text": result.Response})
	if err != nil {
		slog.Error("replay_encode_failed")
		return
	}
	events := []domain.Event{{Name: "message_start", Data: json.RawMessage(`{"replay":true}`)}, {Name: "delta", Data: text}, {Name: "message_end", Data: data}}
	for i, event := range events {
		if i+1 <= after {
			writer.sequence++
			continue
		}
		if err = writer.emit(event); err != nil {
			slog.Info("replay_disconnected")
			return
		}
	}
}
func (a *API) replay(w http.ResponseWriter, r *http.Request) {
	turn, err := a.Store.GetTurn(r.Context(), userID(r), r.PathValue("id"), r.URL.Query().Get("turn_id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	if turn.Status != "completed" {
		fail(w, r, domain.ErrConflict)
		return
	}
	after := 0
	if header := r.Header.Get("Last-Event-ID"); header != "" {
		after, err = strconv.Atoi(header)
		if err != nil || after < 0 || after > 3 {
			fail(w, r, domain.ErrInvalid)
			return
		}
	}
	replayResult(streamWriter(w), *turn.Result, after)
}
func (a *API) memories(w http.ResponseWriter, r *http.Request) {
	var result []domain.Memory
	var err error
	if companion := r.URL.Query().Get("companion_id"); companion != "" {
		result, err = a.Store.CompanionMemories(r.Context(), userID(r), companion)
	} else {
		result, err = a.Store.ListMemories(r.Context(), userID(r))
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) updateMemory(w http.ResponseWriter, r *http.Request) {
	var body domain.MemoryUpdate
	if err := decode(w, r, &body); err != nil || len([]rune(body.Content)) > 1000 || strings.TrimSpace(body.Content) == "" || (body.ConsentStatus != "approved" && body.ConsentStatus != "rejected" && body.ConsentStatus != "pending") {
		fail(w, r, domain.ErrInvalid)
		return
	}
	if body.ConsentStatus == "rejected" {
		if err := a.Store.DeleteMemory(r.Context(), userID(r), r.PathValue("id")); err != nil {
			fail(w, r, err)
			return
		}
		write(w, r, 200, nil)
		return
	}
	result, err := a.Store.UpdateMemory(r.Context(), userID(r), r.PathValue("id"), body)
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, result)
}
func (a *API) deleteMemory(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeleteMemory(r.Context(), userID(r), r.PathValue("id")); err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, nil)
}
func (a *API) deleteMemories(w http.ResponseWriter, r *http.Request) {
	var err error
	if companion := r.URL.Query().Get("companion_id"); companion != "" {
		err = a.Store.DeleteCompanionMemories(r.Context(), userID(r), companion)
	} else {
		err = a.Store.DeleteMemories(r.Context(), userID(r))
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 200, nil)
}
func (a *API) feedback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MessageID string `json:"message_id"`
		Rating    int    `json:"rating"`
		Tag       string `json:"tag"`
	}
	if err := decode(w, r, &body); err != nil || (body.Rating != 1 && body.Rating != -1) || len(body.Tag) > 80 {
		fail(w, r, domain.ErrInvalid)
		return
	}
	if err := a.Store.Feedback(r.Context(), userID(r), body.MessageID, body.Rating, body.Tag); err != nil {
		fail(w, r, err)
		return
	}
	write(w, r, 201, nil)
}
