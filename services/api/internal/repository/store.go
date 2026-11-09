package repository

import (
	"context"
	"encoding/json"

	"companion/api/internal/domain"
)

type Store interface {
	CreateUser(context.Context, string, string) (domain.User, error)
	FindUser(context.Context, string) (domain.User, error)
	GetUser(context.Context, string) (domain.User, error)
	SetMemoryConsent(context.Context, string, bool) error
	GetPersona(context.Context, string) (domain.Persona, error)
	UpdatePersona(context.Context, string, domain.Persona) (domain.Persona, error)
	GetCompanion(context.Context, string, string) (domain.Persona, error)
	ListCompanions(context.Context, string) ([]domain.Persona, error)
	CreateCompanion(context.Context, string, string, string) (domain.Persona, error)
	UpdateCompanion(context.Context, string, string, domain.Persona) (domain.Persona, error)
	ActivateCompanion(context.Context, string, string) (domain.Persona, error)
	ListCharacterCards(context.Context, string) ([]domain.CharacterCard, error)
	CreateCharacterCard(context.Context, string, domain.CharacterCard) (domain.CharacterCard, error)
	ConversationPersona(context.Context, string, string) (domain.Persona, error)
	CreateCompanionConversation(context.Context, string, string, string) (domain.Conversation, error)
	GetCompanionState(context.Context, string, string) (json.RawMessage, error)
	CompanionMemories(context.Context, string, string) ([]domain.Memory, error)
	RelevantCompanionMemories(context.Context, string, string, string) ([]domain.Memory, error)
	ListConversations(context.Context, string) ([]domain.Conversation, error)
	CreateConversation(context.Context, string, string) (domain.Conversation, error)
	Messages(context.Context, string, string) ([]domain.Message, error)
	BeginTurn(context.Context, string, string, string, string) (domain.Turn, error)
	GetTurn(context.Context, string, string, string) (domain.Turn, error)
	FinishTurn(context.Context, string, string, domain.AgentResult) error
	FailTurn(context.Context, string, string) error
	GetState(context.Context, string) (json.RawMessage, error)
	ListMemories(context.Context, string) ([]domain.Memory, error)
	RelevantMemories(context.Context, string, string) ([]domain.Memory, error)
	UpdateMemory(context.Context, string, string, domain.MemoryUpdate) (domain.Memory, error)
	DeleteMemory(context.Context, string, string) error
	DeleteMemories(context.Context, string) error
	DeleteCompanionMemories(context.Context, string, string) error
	Feedback(context.Context, string, string, int, string) error
	Export(context.Context, string) (domain.AccountExport, error)
	DeleteAccount(context.Context, string) error
	GetNotificationPreferences(context.Context, string) (domain.NotificationPreferences, error)
	SetNotificationPreferences(context.Context, string, domain.NotificationPreferences) error
	PauseNotifications(context.Context, string) error
	ListEvents(context.Context, string) ([]domain.ScheduledEvent, error)
	CreateEvent(context.Context, string, domain.ScheduledEvent) (domain.ScheduledEvent, error)
	CancelEvent(context.Context, string, string) error
	ListNotifications(context.Context, string) ([]domain.Notification, error)
}

type Agent interface {
	Respond(context.Context, domain.AgentRequest, func(domain.Event) error) (domain.AgentResult, error)
}
type Limiter interface {
	Allow(context.Context, string, int) (bool, error)
}

type ToolGateway interface {
	ListTools(context.Context, string) ([]domain.ToolDescriptor, error)
	InvokeTool(context.Context, domain.ToolCall) (json.RawMessage, error)
}

type AudioGateway interface {
	Transcribe(context.Context, domain.TranscriptionRequest) (domain.Transcription, error)
	Speech(context.Context, domain.SpeechRequest) ([]byte, error)
}
