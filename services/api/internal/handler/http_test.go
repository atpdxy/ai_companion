package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"companion/api/internal/domain"
)

func TestDecodeRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"name":"a","unexpected":true}`, `{"name":"a"} {}`, `{"name":`} {
		request := httptest.NewRequest("POST", "/", strings.NewReader(body))
		var input struct {
			Name string `json:"name"`
		}
		if err := decode(httptest.NewRecorder(), request, &input); err != domain.ErrInvalid {
			t.Fatalf("accepted invalid body %s", body)
		}
	}
}
func TestReplayStripsTransientEmotion(t *testing.T) {
	recorder := httptest.NewRecorder()
	result := domain.AgentResult{TurnID: "turn_test", Response: "你好", Emotion: json.RawMessage(`{"label":"sadness"}`), Candidates: []domain.Candidate{{Content: "private"}}}
	replayResult(streamWriter(recorder), result, 0)
	if strings.Contains(recorder.Body.String(), "sadness") || strings.Contains(recorder.Body.String(), "private") {
		t.Fatal("transient emotion or candidates persisted in replay")
	}
	if !strings.Contains(recorder.Body.String(), "event: message_end") {
		t.Fatal("no completion")
	}
}
func TestCrossOriginRejectedBeforeAuth(t *testing.T) {
	a := &API{Origin: "http://localhost:5173"}
	request := httptest.NewRequest("POST", "/api/v1/conversations", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://attacker.invalid")
	result := httptest.NewRecorder()
	a.Routes().ServeHTTP(result, request)
	if result.Code != 401 {
		t.Fatalf("status: %d", result.Code)
	}
}
