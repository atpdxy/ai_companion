package infrastructure

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"companion/api/internal/domain"
)

type AgentClient struct {
	URL    string
	Token  string
	Client *http.Client
}

func NewAgentClient(url, token string) *AgentClient {
	return &AgentClient{url, token, &http.Client{Timeout: 140 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (a *AgentClient) Respond(ctx context.Context, input domain.AgentRequest, emit func(domain.Event) error) (domain.AgentResult, error) {
	var result domain.AgentResult
	raw, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	var payload map[string]any
	if err = json.Unmarshal(raw, &payload); err != nil {
		return result, err
	}
	memories := []map[string]any{}
	for _, m := range input.Memories {
		memories = append(memories, map[string]any{"id": m.ID, "content": m.Content, "type": m.Type, "consent_status": m.ConsentStatus, "confidence": m.Confidence, "importance": m.Importance, "expires_at": m.ExpiresAt})
	}
	payload["memories"] = memories
	raw, err = json.Marshal(payload)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.URL, "/")+"/internal/v1/agent/respond", bytes.NewReader(raw))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", input.RequestID)
	response, err := a.Client.Do(req)
	if err != nil {
		return result, fmt.Errorf("AI transport: %w", domain.ErrUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("AI status %d: %w", response.StatusCode, domain.ErrUnavailable)
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 512*1024))
	scanner.Buffer(make([]byte, 4096), 256*1024)
	event := ""
	var full strings.Builder
	completed := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := json.RawMessage(strings.TrimPrefix(line, "data: "))
		if !json.Valid(data) {
			return result, domain.ErrUnavailable
		}
		switch event {
		case "delta":
			var delta struct {
				Text string `json:"text"`
			}
			if err = json.Unmarshal(data, &delta); err != nil {
				return result, err
			}
			full.WriteString(delta.Text)
			if full.Len() > 64000 {
				return result, domain.ErrUnavailable
			}
		case "message_end":
			if err = json.Unmarshal(data, &result); err != nil {
				return result, err
			}
			if result.Response != full.String() || result.TurnID != input.TurnID || !json.Valid(result.State) {
				return result, domain.ErrUnavailable
			}
			completed = true
			continue
		case "error":
			return result, domain.ErrUnavailable
		case "message_start":
		default:
			return result, domain.ErrUnavailable
		}
		if err = emit(domain.Event{Name: event, Data: data}); err != nil {
			return result, err
		}
	}
	if err = scanner.Err(); err != nil {
		return result, err
	}
	if !completed {
		return result, domain.ErrUnavailable
	}
	return result, nil
}
