package infrastructure

import (
	"companion/api/internal/domain"
	"errors"
	"testing"
)

func TestCompanionsHaveIndependentMemoryStateAndPersona(t *testing.T) {
	p, ctx, u := testStore(t)
	lover, err := p.GetPersona(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	cards, err := p.ListCharacterCards(ctx, u.ID)
	if err != nil || len(cards) != 3 {
		t.Fatal("missing preset cards")
	}
	var friendCard string
	for _, card := range cards {
		if card.Definition.Relationship == "friend" {
			friendCard = card.ID
		}
	}
	friend, err := p.CreateCompanion(ctx, u.ID, friendCard, "林舟")
	if err != nil {
		t.Fatal(err)
	}
	if friend.ID == lover.ID || friend.Relationship != "friend" {
		t.Fatal("companion identity not independent")
	}
	if err = p.SetMemoryConsent(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	for _, persona := range []domain.Persona{lover, friend} {
		c, createErr := p.CreateCompanionConversation(ctx, u.ID, persona.ID, "independent")
		if createErr != nil {
			t.Fatal(createErr)
		}
		turn := newID("turn_")
		if _, err = p.BeginTurn(ctx, u.ID, c.ID, turn, "我喜欢咖啡"); err != nil {
			t.Fatal(err)
		}
		result := completedResult(turn)
		result.Candidates = []domain.Candidate{{Content: "我喜欢咖啡", SourceQuote: "我喜欢咖啡", Type: "semantic", Confidence: 1}}
		if err = p.FinishTurn(ctx, u.ID, turn, result); err != nil {
			t.Fatal(err)
		}
		bound, bindErr := p.ConversationPersona(ctx, u.ID, c.ID)
		if bindErr != nil || bound.ID != persona.ID {
			t.Fatal("conversation rebound")
		}
	}
	m, err := p.CompanionMemories(ctx, u.ID, lover.ID)
	if err != nil || len(m) != 1 {
		t.Fatal("lover memory missing")
	}
	other, err := p.CompanionMemories(ctx, u.ID, friend.ID)
	if err != nil || len(other) != 1 || other[0].ID == m[0].ID {
		t.Fatal("memory dedupe crossed companion boundary")
	}
	if err = p.DeleteCompanionMemories(ctx, u.ID, friend.ID); err != nil {
		t.Fatal(err)
	}
	m, err = p.CompanionMemories(ctx, u.ID, lover.ID)
	if err != nil || len(m) != 1 {
		t.Fatal("clearing friend erased lover memory")
	}
	friend.Name = "阿舟"
	if _, err = p.UpdateCompanion(ctx, u.ID, friend.ID, friend); err != nil {
		t.Fatal(err)
	}
	unchanged, err := p.GetCompanion(ctx, u.ID, lover.ID)
	if err != nil || unchanged.Name != lover.Name {
		t.Fatal("persona leaked across companions")
	}
	if _, err = p.GetCompanion(ctx, "another-user", friend.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user companion exposed")
	}
	if _, err = p.CreateCompanion(ctx, "another-user", friendCard, "stolen"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user template copied")
	}
}
