import asyncio
import json
import logging

from pydantic import ValidationError

from emotional_agent.infrastructure.llm import ModelError, ModelGateway
from emotional_agent.personality.prompts import EMOTION_PROMPT
from emotional_agent.schemas.models import ChatMessage, EmotionAnalysis

logger = logging.getLogger(__name__)


async def analyze(
    gateway: ModelGateway, message: str, context: list[ChatMessage], timeout_seconds: float = 12
) -> EmotionAnalysis:
    try:
        raw = await asyncio.wait_for(
            gateway.structured(
                EMOTION_PROMPT,
                json.dumps(
                    {"message": message, "context": [m.model_dump() for m in context]},
                    ensure_ascii=False,
                ),
            ),
            timeout=timeout_seconds,
        )
        return EmotionAnalysis.model_validate_json(raw)
    except (ModelError, ValidationError, TimeoutError):
        logger.warning("emotion_degraded", extra={"event": "emotion_degraded"})
        return EmotionAnalysis(label="unknown", intensity=0, confidence=0)
