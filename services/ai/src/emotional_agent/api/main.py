import hmac
import json
import logging
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from typing import Annotated

import httpx
from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.responses import StreamingResponse

from emotional_agent.agents.orchestrator import respond
from emotional_agent.api.audio import router as audio_router
from emotional_agent.api.tools import router as tools_router
from emotional_agent.infrastructure.config import get_settings
from emotional_agent.infrastructure.llm import CompatibleGateway, DemoGateway
from emotional_agent.infrastructure.tracing import trace_id
from emotional_agent.media.gateway import TelecomAudioGateway
from emotional_agent.schemas.models import ExecutionBudgets, RespondRequest
from emotional_agent.tools.mcp_client import MCPRegistry


class JSONFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        return json.dumps(
            {
                "timestamp": datetime.now(UTC).isoformat(),
                "level": record.levelname,
                "service": "ai",
                "event": getattr(record, "event", record.getMessage()),
                "trace_id": getattr(record, "trace_id", trace_id.get()),
                "latency_ms": getattr(record, "latency_ms", None),
            }
        )


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    settings = get_settings()
    app.state.mcp_registry = MCPRegistry.from_file(
        settings.mcp_servers_file, settings.mcp_timeout_seconds
    )
    handler = logging.StreamHandler()
    handler.setFormatter(JSONFormatter())
    logging.getLogger("emotional_agent").addHandler(handler)
    logging.getLogger("emotional_agent").setLevel(logging.INFO)
    async with httpx.AsyncClient(
        base_url=settings.llm_base_url.rstrip("/") + "/"
        if settings.llm_base_url
        else "http://localhost/",
        headers={"Authorization": f"Bearer {settings.llm_api_key}"},
        timeout=httpx.Timeout(settings.llm_timeout_seconds),
        follow_redirects=False,
    ) as client:
        app.state.gateway = (
            DemoGateway() if settings.llm_mode == "demo" else CompatibleGateway(settings, client)
        )
        async with httpx.AsyncClient(
            timeout=httpx.Timeout(settings.audio_timeout_seconds), follow_redirects=False
        ) as audio_client:
            app.state.audio_gateway = TelecomAudioGateway(settings, audio_client)
            yield


app = FastAPI(title="Companion Internal AI API", version="0.2.0", lifespan=lifespan)
app.include_router(audio_router)
app.include_router(tools_router)


@app.get("/healthz")
async def health() -> dict[str, str]:
    return {"status": "ok", "mode": get_settings().llm_mode}


@app.post("/internal/v1/agent/respond")
async def agent_respond(
    body: RespondRequest,
    request: Request,
    authorization: Annotated[str, Header()] = "",
) -> StreamingResponse:
    settings = get_settings()
    if not hmac.compare_digest(authorization, f"Bearer {settings.service_token}"):
        raise HTTPException(status_code=401, detail="invalid_service_token")

    async def events() -> AsyncIterator[str]:
        token = trace_id.set(body.request_id)
        budgets = ExecutionBudgets(
            emotion_seconds=settings.emotion_timeout_seconds,
            generation_seconds=settings.generation_timeout_seconds,
            memory_seconds=settings.memory_timeout_seconds,
        )
        try:
            async for event, data in respond(
                request.app.state.gateway, body, settings.llm_mode, budgets
            ):
                yield f"event: {event}\ndata: {json.dumps(data, ensure_ascii=False)}\n\n"
        finally:
            trace_id.reset(token)

    return StreamingResponse(
        events(), media_type="text/event-stream", headers={"Cache-Control": "no-store"}
    )
