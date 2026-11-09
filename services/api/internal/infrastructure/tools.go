package infrastructure

import (
	"bytes"
	"companion/api/internal/domain"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (a *AgentClient) toolRequest(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	var raw []byte
	var err error
	if payload != nil {
		raw, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.URL, "/")+"/internal/v1/tools"+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/json")
	response, err := a.Client.Do(req)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == 400 || response.StatusCode == 422 {
		return nil, domain.ErrInvalid
	}
	if response.StatusCode != 200 {
		return nil, domain.ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil {
		return nil, err
	}
	if len(data) > 65536 || !json.Valid(data) {
		return nil, domain.ErrUnavailable
	}
	return data, nil
}
func (a *AgentClient) ListTools(ctx context.Context, user string) ([]domain.ToolDescriptor, error) {
	var result []domain.ToolDescriptor
	raw, err := a.toolRequest(ctx, "GET", "?user_id="+url.QueryEscape(user), nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &result)
	return result, err
}
func (a *AgentClient) InvokeTool(ctx context.Context, call domain.ToolCall) (json.RawMessage, error) {
	return a.toolRequest(ctx, "POST", "/invoke", call)
}
