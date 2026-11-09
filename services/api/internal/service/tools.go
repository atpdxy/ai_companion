package service

import (
	"companion/api/internal/domain"
	"companion/api/internal/repository"
	"context"
	"encoding/json"
)

type Tools struct {
	Store   repository.Store
	Gateway repository.ToolGateway
}

func (t *Tools) List(ctx context.Context, user, companion string) ([]domain.ToolDescriptor, error) {
	if _, err := t.Store.GetCompanion(ctx, user, companion); err != nil {
		return nil, err
	}
	return t.Gateway.ListTools(ctx, user)
}
func (t *Tools) Invoke(ctx context.Context, user, companion, tool, trace string, arguments map[string]any) (json.RawMessage, error) {
	if _, err := t.Store.GetCompanion(ctx, user, companion); err != nil {
		return nil, err
	}
	return t.Gateway.InvokeTool(ctx, domain.ToolCall{UserID: user, ToolID: tool, RequestID: trace, Arguments: arguments})
}
