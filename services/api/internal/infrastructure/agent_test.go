package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"companion/api/internal/domain"
)

func TestAgentContractAndPartialFailure(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(fmt.Sprint(complete), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer service-test" {
					t.Error("service token missing")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["turn_id"] != "turn-test" || body["memories"] == nil {
					t.Error("internal schema missing fields")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if _, err := fmt.Fprint(w, "event: message_start\ndata: {}\n\nevent: delta\ndata: {\"text\":\"hello\"}\n\n"); err != nil {
					t.Error(err)
				}
				if complete {
					if _, err := fmt.Fprint(w, "event: message_end\ndata: {\"turn_id\":\"turn-test\",\"response\":\"hello\",\"state\":{},\"mode\":\"demo\"}\n\n"); err != nil {
						t.Error(err)
					}
				}
			}))
			defer server.Close()
			var deltas int
			client := NewAgentClient(server.URL, "service-test")
			result, err := client.Respond(context.Background(), domain.AgentRequest{TurnID: "turn-test", State: json.RawMessage(`{}`)}, func(event domain.Event) error {
				if event.Name == "delta" {
					deltas++
				}
				if event.Name == "message_end" {
					t.Error("completion must wait for durable commit")
				}
				return nil
			})
			if complete && (err != nil || result.Response != "hello" || deltas != 1) {
				t.Fatalf("bad successful response: %v", err)
			}
			if !complete && err == nil {
				t.Fatal("incomplete upstream accepted")
			}
		})
	}
}
