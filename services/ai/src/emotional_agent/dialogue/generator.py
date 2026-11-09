import json
from collections.abc import AsyncIterator

from emotional_agent.infrastructure.llm import ModelError, ModelGateway
from emotional_agent.personality.prompts import GENERATION_PROMPT
from emotional_agent.safety.guard import output_allowed
from emotional_agent.schemas.models import Memory, RespondRequest, StrategyDecision


async def generate(
    gateway: ModelGateway,
    request: RespondRequest,
    decision: StrategyDecision,
    memories: list[Memory],
) -> AsyncIterator[str]:
    if decision.action == "respect_space":
        yield "好，我先不打扰你。按你自己的节奏来。"
        return
    if decision.action == "safety_support":
        yield (
            "听到你这样说，我很重视你此刻的安全。若你有立即伤害自己或他人的打算，"
            "请先远离危险物品，联系身边可信赖的人陪着你，并联系当地紧急服务。"
            "我可以陪你梳理下一步，但不能替代现实中的专业支持。"
        )
        return
    payload = json.dumps(
        {
            "message": request.message,
            "context": [m.model_dump() for m in request.context],
            "persona": request.persona.model_dump(),
            "strategy": decision.model_dump(),
            "memories": [m.model_dump(mode="json") for m in memories],
        },
        ensure_ascii=False,
    )
    # Delay a short suffix so forbidden phrases split across tokens can be rejected.
    pending = ""
    full = ""
    async for token in gateway.stream(GENERATION_PROMPT, payload):
        pending += token
        full += token
        if len(full) > 12000 or not output_allowed(full):
            raise ModelError("output_guard_rejected")
        if len(pending) > 24:
            yield pending[:-24]
            pending = pending[-24:]
    if not full.strip():
        raise ModelError("empty_response")
    if pending:
        yield pending
