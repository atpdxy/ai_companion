package infrastructure

import (
	"companion/api/internal/domain"
	"testing"
)

func TestPostgresClearAllDuringTurnDoesNotRecreateMemory(t *testing.T) {
	p, ctx, u := testStore(t)
	if err := p.SetMemoryConsent(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	c, err := p.CreateConversation(ctx, u.ID, "inflight clear")
	if err != nil {
		t.Fatal(err)
	}
	turn := newID("turn_")
	if _, err = p.BeginTurn(ctx, u.ID, c.ID, turn, "我喜欢咖啡"); err != nil {
		t.Fatal(err)
	}
	if err = p.DeleteMemories(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	result := completedResult(turn)
	result.Candidates = []domain.Candidate{{Content: "我喜欢咖啡", Type: "semantic", Confidence: 1, SourceQuote: "我喜欢咖啡"}}
	if err = p.FinishTurn(ctx, u.ID, turn, result); err != nil {
		t.Fatal(err)
	}
	memories, err := p.ListMemories(ctx, u.ID)
	if err != nil || len(memories) != 0 {
		t.Fatal("cleared memory recreated by an inflight turn")
	}
}

func TestPostgresExportDoesNotTruncateConversationHistory(t *testing.T) {
	p, ctx, u := testStore(t)
	if _, err := p.Pool.Exec(ctx, `INSERT INTO conversations(id,user_id,title,companion_id) SELECT $1 || '-' || i::text,$1,'export test',(SELECT active_companion_id FROM users WHERE id=$1) FROM generate_series(1,205) i`, u.ID); err != nil {
		t.Fatal(err)
	}
	visible, err := p.ListConversations(ctx, u.ID)
	if err != nil || len(visible) != 200 {
		t.Fatal("unexpected list limit")
	}
	export, err := p.Export(ctx, u.ID)
	if err != nil || len(export.Conversations) != 205 {
		t.Fatalf("export truncated: %v", err)
	}
}
