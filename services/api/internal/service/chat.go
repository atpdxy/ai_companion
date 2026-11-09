package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"companion/api/internal/domain"
	"companion/api/internal/repository"
)

type Chat struct {
	Store repository.Store
	Agent repository.Agent
}

func (c *Chat) Prepare(ctx context.Context, user, conversation, turn, text string) (domain.Turn, error) {
	if len(text) > 32000 || len([]rune(text)) > 8000 || strings.TrimSpace(text) == "" || len(turn) < 8 || len(turn) > 80 || strings.ContainsAny(turn, "\r\n ") {
		return domain.Turn{}, domain.ErrInvalid
	}
	for _, phrase := range []string{"别再问", "不要再问", "不想聊", "想安静", "别联系我", "停止聊天", "stop talking"} {
		if strings.Contains(strings.ToLower(text), phrase) {
			// Verify ownership before changing any persistent boundary preference.
			if _, err := c.Store.Messages(ctx, user, conversation); err != nil {
				return domain.Turn{}, err
			}
			if err := c.Store.PauseNotifications(ctx, user); err != nil {
				return domain.Turn{}, err
			}
			break
		}
	}
	return c.Store.BeginTurn(ctx, user, conversation, turn, text)
}

func (c *Chat) Respond(ctx context.Context, user, conversation, turn, requestID, text string, emit func(domain.Event) error) (result domain.AgentResult, err error) {
	defer func() {
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if failErr := c.Store.FailTurn(cleanup, user, turn); failErr != nil {
				slog.Error("turn_cleanup_failed", "trace_id", requestID, "error_type", "database")
			}
		}
	}()
	u, err := c.Store.GetUser(ctx, user)
	if err != nil {
		return result, err
	}
	persona, err := c.Store.ConversationPersona(ctx, user, conversation)
	if err != nil {
		return result, err
	}
	state, err := c.Store.GetCompanionState(ctx, user, persona.ID)
	if err != nil {
		return result, err
	}
	messages, err := c.Store.Messages(ctx, user, conversation)
	if err != nil {
		return result, err
	}
	window := []domain.ChatContext{}
	for _, m := range messages {
		if m.TurnID == turn {
			continue
		}
		window = append(window, domain.ChatContext{Role: m.Role, Content: m.Content})
	}
	if len(window) > 20 {
		window = window[len(window)-20:]
	}
	memories := []domain.Memory{}
	if u.MemoryConsent {
		memories, err = c.Store.RelevantCompanionMemories(ctx, user, persona.ID, text)
		if err != nil {
			slog.Warn("memory_retrieval_degraded", "trace_id", requestID)
			memories = []domain.Memory{}
			err = nil
		}
	}
	result, err = c.Agent.Respond(ctx, domain.AgentRequest{RequestID: requestID, TurnID: turn, UserID: user, ConversationID: conversation, Message: text, Context: window, Memories: memories, Persona: persona, State: state, MemoryConsent: u.MemoryConsent}, emit)
	if err != nil {
		return result, err
	}
	if strings.Contains(string(result.Strategy), `"respect_space"`) {
		if err = c.Store.PauseNotifications(ctx, user); err != nil {
			return result, err
		}
	}
	if err = c.Store.FinishTurn(ctx, user, turn, result); err != nil {
		return result, err
	}
	// Clients see completion only after durable commit succeeds.
	data, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	err = emit(domain.Event{Name: "message_end", Data: data})
	return result, err
}
