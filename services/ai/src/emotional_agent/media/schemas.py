import base64
import binascii
from typing import Literal

from pydantic import Field, model_validator

from emotional_agent.schemas.models import Schema

AudioFormat = Literal["wav", "mp3", "webm", "ogg", "opus", "flac", "m4a", "mp4", "aac"]
MIME_TYPES = {
    "wav": "audio/wav",
    "mp3": "audio/mpeg",
    "webm": "audio/webm",
    "ogg": "audio/ogg",
    "opus": "audio/opus",
    "flac": "audio/flac",
    "m4a": "audio/mp4",
    "mp4": "audio/mp4",
    "aac": "audio/aac",
}


class TranscriptionRequest(Schema):
    audio_base64: str = Field(min_length=4, max_length=14_000_000)
    format: AudioFormat
    request_id: str = Field(min_length=1, max_length=80)

    @model_validator(mode="after")
    def validate_audio(self) -> "TranscriptionRequest":
        try:
            raw = base64.b64decode(self.audio_base64, validate=True)
        except (ValueError, binascii.Error) as error:
            raise ValueError("invalid_audio_base64") from error
        if not raw or len(raw) > 10 * 1024 * 1024:
            raise ValueError("audio_too_large_or_empty")
        valid = {
            "wav": raw.startswith(b"RIFF") and raw[8:12] == b"WAVE",
            "webm": raw.startswith(b"\x1aE\xdf\xa3"),
            "ogg": raw.startswith(b"OggS"),
            "opus": raw.startswith(b"OggS"),
            "flac": raw.startswith(b"fLaC"),
            "mp4": raw[4:8] == b"ftyp",
            "m4a": raw[4:8] == b"ftyp",
            "mp3": raw.startswith(b"ID3") or raw[:1] == b"\xff",
            "aac": raw[:1] == b"\xff",
        }
        if not valid[self.format]:
            raise ValueError("audio_format_mismatch")
        return self


class TranscriptionResult(Schema):
    text: str = Field(min_length=1, max_length=16000)
    duration_seconds: float | None = Field(default=None, ge=0, le=300)


class SpeechRequest(Schema):
    text: str = Field(min_length=1, max_length=1000)
    voice_id: str = Field(default="", max_length=100)
    request_id: str = Field(min_length=1, max_length=80)


class TTSEvent(Schema):
    model_config = {"extra": "ignore"}
    code: int
    data: str | None = None
