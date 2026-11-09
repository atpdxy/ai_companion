package domain

type TranscriptionRequest struct {
	AudioBase64 string `json:"audio_base64"`
	Format      string `json:"format"`
	RequestID   string `json:"request_id"`
}
type Transcription struct {
	Text            string   `json:"text"`
	DurationSeconds *float64 `json:"duration_seconds"`
}
type SpeechRequest struct {
	Text      string `json:"text"`
	VoiceID   string `json:"voice_id"`
	RequestID string `json:"request_id"`
}
