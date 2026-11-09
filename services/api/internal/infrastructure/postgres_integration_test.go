package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"companion/api/internal/domain"
)

func testStore(t *testing.T) (*Postgres, context.Context, domain.User) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	p, err := NewPostgres(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Pool.Close)
	if err = p.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u, err := p.CreateUser(ctx, newID("integration_")+"@example.invalid", "unused-test-hash")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.DeleteAccount(ctx, u.ID); err != nil {
			t.Error(err)
		}
	})
	return p, ctx, u
}
func completedResult(turn string) domain.AgentResult {
	return domain.AgentResult{TurnID: turn, Response: "听到了", State: json.RawMessage(`{"valence":0.1,"arousal":0.2,"familiarity":0.01,"interaction_energy":0.8}`), Strategy: json.RawMessage(`{"action":"listen","reason_code":"test"}`), Emotion: json.RawMessage(`{"label":"sadness","confidence":0.8}`), Mode: "demo"}
}

func TestPostgresTurnIdempotencyIsolationAndMemoryLifecycle(t *testing.T) {
	p, ctx, u := testStore(t)
	conversation, err := p.CreateConversation(ctx, u.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	turn := newID("turn_")
	message := "请记住：我喜欢下雨天"
	first, err := p.BeginTurn(ctx, u.ID, conversation.ID, turn, message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.BeginTurn(ctx, u.ID, conversation.ID, turn, message); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("concurrent turn accepted")
	}
	if _, err = p.Messages(ctx, "another-user", conversation.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user timeline exposed")
	}
	if _, err = p.GetTurn(ctx, "another-user", conversation.ID, turn); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user replay exposed")
	}
	if err = p.SetMemoryConsent(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	result := completedResult(turn)
	result.Candidates = []domain.Candidate{{Content: "我喜欢下雨天", Type: "semantic", Confidence: 1, SourceQuote: "我喜欢下雨天"}}
	if err = p.FinishTurn(ctx, u.ID, turn, result); err != nil {
		t.Fatal(err)
	}
	replay, err := p.BeginTurn(ctx, u.ID, conversation.ID, turn, message)
	if err != nil || replay.Status != "completed" {
		t.Fatalf("replay failed: %v", err)
	}
	if len(replay.Result.Emotion) > 0 {
		t.Fatal("sensitive emotion persisted")
	}
	if _, err = p.BeginTurn(ctx, u.ID, conversation.ID, turn, "changed"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("reused key accepted different content")
	}
	timeline, err := p.Messages(ctx, u.ID, conversation.ID)
	if err != nil || len(timeline) != 2 || timeline[0].ID != first.UserMessageID {
		t.Fatalf("duplicated timeline: %v", err)
	}
	memories, err := p.ListMemories(ctx, u.ID)
	if err != nil || len(memories) != 1 {
		t.Fatalf("no pending candidate: %v", err)
	}
	memory := memories[0]
	matched, err := p.RelevantMemories(ctx, u.ID, "下雨天")
	if err != nil || len(matched) != 0 {
		t.Fatal("pending memory recalled")
	}
	if _, err = p.UpdateMemory(ctx, "another-user", memory.ID, domain.MemoryUpdate{Content: "stolen", ConsentStatus: "approved"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user mutation allowed")
	}
	if _, err = p.UpdateMemory(ctx, u.ID, memory.ID, domain.MemoryUpdate{Content: "我喜欢晴天", ConsentStatus: "approved"}); err != nil {
		t.Fatal(err)
	}
	matched, err = p.RelevantMemories(ctx, u.ID, "晴天")
	if err != nil || len(matched) != 1 || matched[0].Content != "我喜欢晴天" {
		t.Fatalf("correction not indexed: %v", err)
	}
	if err = p.DeleteMemory(ctx, "another-user", memory.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user deletion allowed")
	}
	if err = p.DeleteMemory(ctx, u.ID, memory.ID); err != nil {
		t.Fatal(err)
	}
	matched, err = p.RelevantMemories(ctx, u.ID, "晴天")
	if err != nil || len(matched) != 0 {
		t.Fatal("deleted memory recalled")
	}
	if err = p.Feedback(ctx, "another-user", timeline[1].ID, 1, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user feedback accepted")
	}
}

func TestPostgresRevocationDuringTurnPreventsCandidateWrite(t *testing.T) {
	p, ctx, u := testStore(t)
	c, err := p.CreateConversation(ctx, u.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err = p.SetMemoryConsent(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	turn := newID("turn_")
	if _, err = p.BeginTurn(ctx, u.ID, c.ID, turn, "我喜欢咖啡"); err != nil {
		t.Fatal(err)
	}
	if err = p.SetMemoryConsent(ctx, u.ID, false); err != nil {
		t.Fatal(err)
	}
	result := completedResult(turn)
	result.Candidates = []domain.Candidate{{Content: "我喜欢咖啡", SourceQuote: "我喜欢咖啡", Type: "semantic", Confidence: 1}}
	if err = p.FinishTurn(ctx, u.ID, turn, result); err != nil {
		t.Fatal(err)
	}
	m, err := p.ListMemories(ctx, u.ID)
	if err != nil || len(m) != 0 {
		t.Fatal("candidate written after consent revoked")
	}
}

func TestPostgresSchedulerRechecksConsentAndDeduplicates(t *testing.T) {
	p, ctx, u := testStore(t)
	now := time.Now().UTC()
	prefs := domain.DefaultNotificationPreferences()
	prefs.OptIn = true
	prefs.QuietStart = 0
	prefs.QuietEnd = 0
	prefs.DailyLimit = 1
	if err := p.SetNotificationPreferences(ctx, u.ID, prefs); err != nil {
		t.Fatal(err)
	}
	event := domain.ScheduledEvent{Content: "带上面试材料", DueAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour), Recurrence: "none", ClientKey: newID("client_")}
	created, err := p.CreateEvent(ctx, u.ID, event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.CreateEvent(ctx, u.ID, event); err != nil {
		t.Fatal(err)
	}
	prefs.OptIn = false
	if err = p.SetNotificationPreferences(ctx, u.ID, prefs); err != nil {
		t.Fatal(err)
	}
	if err = p.DeliverDue(ctx, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = p.Pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1`, u.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("delivered after opt out")
	}
	prefs.OptIn = true
	if err = p.SetNotificationPreferences(ctx, u.ID, prefs); err != nil {
		t.Fatal(err)
	}
	if err = p.DeliverDue(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err = p.DeliverDue(ctx, now); err != nil {
		t.Fatal(err)
	}
	notifications, err := p.ListNotifications(ctx, u.ID)
	if err != nil || len(notifications) != 1 || notifications[0].EventID != created.ID {
		t.Fatalf("delivery duplicated or missing: %v", err)
	}
	event.ClientKey = newID("client_")
	if _, err = p.CreateEvent(ctx, u.ID, event); err != nil {
		t.Fatal(err)
	}
	if err = p.DeliverDue(ctx, now); err != nil {
		t.Fatal(err)
	}
	notifications, err = p.ListNotifications(ctx, u.ID)
	if err != nil || len(notifications) != 1 {
		t.Fatal("quota not enforced")
	}
	if err = p.PauseNotifications(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	prefs, err = p.GetNotificationPreferences(ctx, u.ID)
	if err != nil || prefs.OptIn || !prefs.NoContact {
		t.Fatal("stop request not persisted")
	}
}
