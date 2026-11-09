package domain

import "encoding/json"

type ToolDescriptor struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}
type ToolCall struct {
	UserID    string         `json:"user_id"`
	ToolID    string         `json:"tool_id"`
	RequestID string         `json:"request_id"`
	Arguments map[string]any `json:"arguments"`
}
