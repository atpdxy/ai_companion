package infrastructure

import (
	"bytes"
	"companion/api/internal/domain"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (a *AgentClient) audioRequest(ctx context.Context, path string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.URL, "/")+"/internal/v1/audio/"+path, bytes.NewReader(raw))
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
	if response.StatusCode == 422 || response.StatusCode == 400 {
		return nil, domain.ErrInvalid
	}
	if response.StatusCode != 200 {
		return nil, domain.ErrUnavailable
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(content) > 16*1024*1024 {
		return nil, domain.ErrUnavailable
	}
	return content, nil
}
func (a *AgentClient) Transcribe(ctx context.Context, input domain.TranscriptionRequest) (domain.Transcription, error) {
	var result domain.Transcription
	raw, err := a.audioRequest(ctx, "transcriptions", input)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	if err == nil && strings.TrimSpace(result.Text) == "" {
		err = domain.ErrUnavailable
	}
	return result, err
}
func (a *AgentClient) Speech(ctx context.Context, input domain.SpeechRequest) ([]byte, error) {
	return a.audioRequest(ctx, "speech", input)
}
