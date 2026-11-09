import hmac
import logging
from typing import Annotated

from fastapi import APIRouter, Header, HTTPException, Request
from fastapi.responses import Response

from emotional_agent.infrastructure.config import get_settings
from emotional_agent.media.gateway import AudioError, TelecomAudioGateway
from emotional_agent.media.schemas import SpeechRequest, TranscriptionRequest, TranscriptionResult

router = APIRouter(prefix="/internal/v1/audio")
logger = logging.getLogger(__name__)


def authorize(authorization: str) -> None:
    if not hmac.compare_digest(authorization, f"Bearer {get_settings().service_token}"):
        raise HTTPException(status_code=401, detail="invalid_service_token")


@router.post("/transcriptions")
async def transcription(
    body: TranscriptionRequest, request: Request, authorization: Annotated[str, Header()] = ""
) -> TranscriptionResult:
    authorize(authorization)
    gateway: TelecomAudioGateway = request.app.state.audio_gateway
    try:
        return await gateway.transcribe(body)
    except AudioError as error:
        logger.warning("asr_failed", extra={"event": "asr_failed", "trace_id": body.request_id})
        raise HTTPException(status_code=503, detail="asr_unavailable") from error


@router.post("/speech")
async def speech(
    body: SpeechRequest, request: Request, authorization: Annotated[str, Header()] = ""
) -> Response:
    authorize(authorization)
    gateway: TelecomAudioGateway = request.app.state.audio_gateway
    try:
        audio = await gateway.synthesize(body)
        return Response(
            content=audio, media_type="audio/mpeg", headers={"Cache-Control": "no-store"}
        )
    except AudioError as error:
        logger.warning("tts_failed", extra={"event": "tts_failed", "trace_id": body.request_id})
        raise HTTPException(status_code=503, detail="tts_unavailable") from error
