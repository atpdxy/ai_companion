import asyncio
import logging
import time
from collections.abc import AsyncIterator
from typing import Any

from emotional_agent.agents.planner import decide
from emotional_agent.dialogue.generator import generate
from emotional_agent.emotion.analyzer import analyze
from emotional_agent.infrastructure.llm import ModelError, ModelGateway
from emotional_agent.memory.manager import extract, retrieve
from emotional_agent.schemas.models import ExecutionBudgets, RespondRequest
from emotional_agent.state.engine import next_state

logger = logging.getLogger(__name__)


async def respond(
    gateway: ModelGateway,
    request: RespondRequest,
    mode: str,
    budgets: ExecutionBudgets | None = None,
) -> AsyncIterator[tuple[str, dict[str, Any]]]:
    started = time.monotonic()
    budgets = budgets or ExecutionBudgets()
    emotion, memories = await asyncio.gather(
        analyze(gateway, request.message, request.context, budgets.emotion_seconds),
        asyncio.to_thread(
            retrieve, request.memories if request.memory_consent else [], request.message
        ),
    )
    decision = decide(request.message, emotion)
    yield "message_start", {"turn_id": request.turn_id, "mode": mode}
    response = ""
    try:
        async with asyncio.timeout(budgets.generation_seconds):
            async for token in generate(gateway, request, decision, memories):
                response += token
                yield "delta", {"text": token}
    except (ModelError, TimeoutError):
        logger.warning(
            "generation_failed",
            extra={"event": "generation_failed", "trace_id": request.request_id},
        )
        yield "error", {"code": "AI_UNAVAILABLE", "message": "回复未完成，请使用同一轮次重试。"}
        return
    candidates = []
    if request.memory_consent and decision.action not in {"respect_space", "safety_support"}:
        candidates = await extract(gateway, request.message, budgets.memory_seconds)
    latency = round((time.monotonic() - started) * 1000)
    yield (
        "message_end",
        {
            "turn_id": request.turn_id,
            "response": response,
            "emotion": emotion.model_dump(),
            "strategy": decision.model_dump(),
            "state": next_state(request.state, emotion).model_dump(mode="json"),
            "retrieved_memory_ids": [memory.id for memory in memories],
            "candidates": [candidate.model_dump() for candidate in candidates],
            "persona_version": request.persona.version,
            "prompt_version": "dialogue.v2/emotion.v1/memory.v1",
            "latency_ms": latency,
            "mode": mode,
        },
    )
    logger.info(
        "turn_completed",
        extra={
            "event": "turn_completed",
            "trace_id": request.request_id,
            "latency_ms": latency,
            "strategy": decision.action,
        },
    )
