package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalid      = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrUnavailable  = errors.New("service unavailable")
)

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	MemoryConsent bool      `json:"memory_consent"`
	CreatedAt     time.Time `json:"created_at"`
}

type Persona struct {
	ID           string  `json:"id,omitempty"`
	CardID       string  `json:"card_id,omitempty"`
	Relationship string  `json:"relationship"`
	Description  string  `json:"description"`
	Greeting     string  `json:"greeting"`
	VoiceID      string  `json:"voice_id"`
	Name         string  `json:"name"`
	UserName     string  `json:"user_name"`
	Warmth       float64 `json:"warmth"`
	Brevity      float64 `json:"brevity"`
	Humor        float64 `json:"humor"`
	Version      int     `json:"version"`
}

func DefaultPersona() Persona {
	return Persona{Name: "知夏", UserName: "你", Relationship: "lover", Description: "温柔、真诚的虚拟爱人，尊重彼此的生活与边界。", Greeting: "今天的你，想从哪里说起？", Warmth: .85, Brevity: .6, Humor: .35, Version: 1}
}
func (p Persona) Valid() bool {
	return len([]rune(p.Name)) >= 1 && len([]rune(p.Name)) <= 24 && len([]rune(p.UserName)) >= 1 && len([]rune(p.UserName)) <= 24 && p.Warmth >= 0 && p.Warmth <= 1 && p.Brevity >= 0 && p.Brevity <= 1 && p.Humor >= 0 && p.Humor <= 1
}

type Conversation struct {
	CompanionID string    `json:"companion_id"`
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
}

type Message struct {
	ID        string    `json:"id"`
	TurnID    string    `json:"turn_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Memory struct {
	CompanionID     string     `json:"companion_id"`
	ID              string     `json:"id"`
	Content         string     `json:"content"`
	Type            string     `json:"type"`
	ConsentStatus   string     `json:"consent_status"`
	Confidence      float64    `json:"confidence"`
	Importance      float64    `json:"importance"`
	SourceMessageID string     `json:"source_message_id"`
	SourceQuote     string     `json:"source_quote"`
	ExpiresAt       *time.Time `json:"expires_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type MemoryUpdate struct {
	Content       string     `json:"content"`
	ConsentStatus string     `json:"consent_status"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

type Candidate struct {
	Content     string  `json:"content"`
	Type        string  `json:"type"`
	Confidence  float64 `json:"confidence"`
	SourceQuote string  `json:"source_quote"`
}

type AgentResult struct {
	TurnID             string          `json:"turn_id"`
	Response           string          `json:"response"`
	Emotion            json.RawMessage `json:"emotion,omitempty"`
	Strategy           json.RawMessage `json:"strategy"`
	State              json.RawMessage `json:"state"`
	RetrievedMemoryIDs []string        `json:"retrieved_memory_ids"`
	Candidates         []Candidate     `json:"candidates"`
	PersonaVersion     int             `json:"persona_version"`
	PromptVersion      string          `json:"prompt_version"`
	LatencyMS          int             `json:"latency_ms"`
	Mode               string          `json:"mode"`
}

type Turn struct {
	ID            string
	UserMessageID string
	Status        string
	Result        *AgentResult
}

type AgentRequest struct {
	RequestID      string          `json:"request_id"`
	TurnID         string          `json:"turn_id"`
	UserID         string          `json:"user_id"`
	ConversationID string          `json:"conversation_id"`
	Message        string          `json:"message"`
	Context        []ChatContext   `json:"context"`
	Memories       []Memory        `json:"-"`
	Persona        Persona         `json:"persona"`
	State          json.RawMessage `json:"state"`
	MemoryConsent  bool            `json:"memory_consent"`
}

type ChatContext struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Event struct {
	Name string
	Data json.RawMessage
}

type AccountExport struct {
	Companions              []Persona               `json:"companions"`
	CharacterCards          []CharacterCard         `json:"character_cards"`
	User                    User                    `json:"user"`
	Persona                 Persona                 `json:"persona"`
	Conversations           []Conversation          `json:"conversations"`
	Messages                []Message               `json:"messages"`
	Memories                []Memory                `json:"memories"`
	NotificationPreferences NotificationPreferences `json:"notification_preferences"`
	Events                  []ScheduledEvent        `json:"events"`
	Notifications           []Notification          `json:"notifications"`
}
