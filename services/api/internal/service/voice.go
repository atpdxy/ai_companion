package service

import (
	"companion/api/internal/domain"
	"companion/api/internal/repository"
	"context"
)

type Voice struct {
	Store   repository.Store
	Gateway repository.AudioGateway
}

func (v *Voice) Transcribe(ctx context.Context, user, companion string, input domain.TranscriptionRequest) (domain.Transcription, error) {
	if _, err := v.Store.GetCompanion(ctx, user, companion); err != nil {
		return domain.Transcription{}, err
	}
	return v.Gateway.Transcribe(ctx, input)
}
func (v *Voice) Speech(ctx context.Context, user, conversation, message, trace string) ([]byte, error) {
	persona, err := v.Store.ConversationPersona(ctx, user, conversation)
	if err != nil {
		return nil, err
	}
	messages, err := v.Store.Messages(ctx, user, conversation)
	if err != nil {
		return nil, err
	}
	for _, item := range messages {
		if item.ID == message && item.Role == "assistant" {
			if len([]rune(item.Content)) > 1000 {
				return nil, domain.ErrInvalid
			}
			return v.Gateway.Speech(ctx, domain.SpeechRequest{Text: item.Content, VoiceID: persona.VoiceID, RequestID: trace})
		}
	}
	return nil, domain.ErrNotFound
}
