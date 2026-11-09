import asyncio
import json
import logging
from datetime import UTC, datetime

from pydantic import ValidationError

from emotional_agent.infrastructure.llm import ModelError, ModelGateway
from emotional_agent.personality.prompts import MEMORY_PROMPT
from emotional_agent.safety.guard import memory_allowed
from emotional_agent.schemas.models import CandidateList, Memory, MemoryCandidate

logger = logging.getLogger(__name__)


def retrieve(memories: list[Memory], message: str) -> list[Memory]:
    now = datetime.now(UTC)
    query = {message[i : i + 2] for i in range(max(0, len(message) - 1))}
    candidates: list[tuple[float, Memory]] = []
    for memory in memories:
        if memory.consent_status != "approved" or memory.confidence < 0.8:
            continue
        if memory.expires_at and memory.expires_at <= now:
            continue
        overlap = sum(term in memory.content for term in query) / max(len(query), 1)
        if overlap > 0 or memory.type == "boundary":
            candidates.append((overlap + 0.1 * memory.importance, memory))
    return [memory for _, memory in sorted(candidates, key=lambda item: item[0], reverse=True)[:5]]


async def extract(
    gateway: ModelGateway, message: str, timeout_seconds: float = 10
) -> list[MemoryCandidate]:
    if not memory_allowed(message):
        return []
    try:
        raw = await asyncio.wait_for(
            gateway.structured(MEMORY_PROMPT, json.dumps({"message": message}, ensure_ascii=False)),
            timeout=timeout_seconds,
        )
        result = CandidateList.model_validate_json(raw)
        return [
            candidate
            for candidate in result.candidates
            if candidate.confidence >= 0.8
            and candidate.source_quote in message
            and memory_allowed(candidate.content)
        ]
    except (ModelError, ValidationError, TimeoutError):
        logger.warning("memory_extraction_degraded", extra={"event": "memory_extraction_degraded"})
        return []
