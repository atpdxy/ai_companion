package domain

import "strings"

const CardSchemaVersion = "companion.character.v1"

// A character card is a reusable template. Each companion receives an independent copy.
type CharacterCard struct {
	ID            string  `json:"id,omitempty"`
	SchemaVersion string  `json:"schema_version"`
	Definition    Persona `json:"definition"`
	Version       int     `json:"version"`
}

func (p Persona) ValidCharacter() bool {
	return p.Valid() && (p.Relationship == "lover" || p.Relationship == "family" || p.Relationship == "friend" || p.Relationship == "custom") && len([]rune(p.Description)) <= 2000 && len([]rune(p.Greeting)) <= 500 && len(p.VoiceID) <= 100 && !strings.ContainsAny(p.VoiceID, "\r\n")
}

func PresetCards() []CharacterCard {
	lover := DefaultPersona()
	friend := lover
	friend.Name = "林舟"
	friend.Relationship = "friend"
	friend.Description = "坦诚、幽默的虚拟朋友，平等交流，支持现实社交。"
	friend.Greeting = "最近怎么样？说说你的新鲜事。"
	friend.Humor = .65
	family := lover
	family.Name = "知秋"
	family.Relationship = "family"
	family.Description = "温和、关怀的虚拟家人，以亲情方式陪伴，不扮演爱人。"
	family.Greeting = "回来了，今天过得怎么样？"
	family.Humor = .2
	return []CharacterCard{{SchemaVersion: CardSchemaVersion, Definition: lover, Version: 1}, {SchemaVersion: CardSchemaVersion, Definition: friend, Version: 1}, {SchemaVersion: CardSchemaVersion, Definition: family, Version: 1}}
}
