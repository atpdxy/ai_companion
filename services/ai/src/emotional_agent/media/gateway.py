import asyncio
import base64
import binascii
import uuid
from typing import Protocol

import httpx
from pydantic import ValidationError

from emotional_agent.infrastructure.config import Settings
from emotional_agent.media.schemas import (
    MIME_TYPES,
    SpeechRequest,
    TranscriptionRequest,
    TranscriptionResult,
    TTSEvent,
)


class AudioError(Exception):
    """Audio provider failure with sanitized diagnostic code only."""


class SpeechRecognition(Protocol):
    async def transcribe(self, request: TranscriptionRequest) -> TranscriptionResult: ...


class SpeechSynthesis(Protocol):
    async def synthesize(self, request: SpeechRequest) -> bytes: ...


class TelecomAudioGateway:
    def __init__(self, settings: Settings, client: httpx.AsyncClient) -> None:
        self.settings = settings
        self.client = client

    async def transcribe(self, request: TranscriptionRequest) -> TranscriptionResult:
        if not self.settings.asr_enabled:
            raise AudioError("asr_not_configured")
        data_uri = f"data:{MIME_TYPES[request.format]};base64,{request.audio_base64}"
        api_key = self.settings.asr_api_key or self.settings.llm_api_key
        payload = {
            "model": self.settings.asr_model,
            "input": {
                "messages": [
                    {
                        "role": "user",
                        "content": [
                            {
                                "type": "input_audio",
                                "input_audio": {"data": data_uri},
                            }
                        ],
                    }
                ]
            },
            "parameters": {"format": request.format},
        }
        try:
            async with asyncio.timeout(self.settings.audio_timeout_seconds):
                response = await self.client.post(
                    self.settings.asr_url,
                    headers={
                        "Authorization": f"Bearer {api_key}",
                        "X-DashScope-SSE": "disable",
                    },
                    json=payload,
                )
                response.raise_for_status()
                data = response.json()
                return TranscriptionResult(
                    text=data["output"]["text"],
                    duration_seconds=data.get("usage", {}).get("duration"),
                )
        except (httpx.HTTPError, ValueError, KeyError, TypeError, TimeoutError) as error:
            raise AudioError("asr_failed") from error

    async def synthesize(self, request: SpeechRequest) -> bytes:
        if not self.settings.tts_enabled:
            raise AudioError("tts_not_configured")
        headers = {
            "X-Api-Key": self.settings.tts_api_key or self.settings.llm_api_key,
            "X-Api-Resource-Id": self.settings.tts_model,
            "X-Api-Request-Id": str(uuid.uuid4()),
        }
        payload = {
            "req_params": {
                "text": request.text,
                "speaker": request.voice_id or self.settings.tts_speaker,
                "audio_params": {"format": "mp3", "sample_rate": 24000, "speech_rate": 0},
            }
        }
        audio = bytearray()
        complete = False
        try:
            async with asyncio.timeout(self.settings.audio_timeout_seconds):
                async with self.client.stream(
                    "POST", self.settings.tts_url, headers=headers, json=payload
                ) as response:
                    response.raise_for_status()
                    async for line in response.aiter_lines():
                        if not line.strip():
                            continue
                        if len(line) > 4_000_000:
                            raise AudioError("tts_event_too_large")
                        event = TTSEvent.model_validate_json(line)
                        if event.code == 20000000:
                            complete = True
                            break
                        if event.code != 0:
                            raise AudioError("tts_business_failure")
                        if event.data:
                            audio.extend(base64.b64decode(event.data, validate=True))
                            if len(audio) > 15 * 1024 * 1024:
                                raise AudioError("tts_audio_too_large")
            if not complete or not audio:
                raise AudioError("tts_incomplete")
            return bytes(audio)
        except (httpx.HTTPError, ValidationError, binascii.Error, TimeoutError) as error:
            raise AudioError("tts_failed") from error
