from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta

import httpx
import pytest
from fastapi.testclient import TestClient
from pydantic import ValidationError

from emotional_agent.agents.orchestrator import respond
from emotional_agent.agents.planner import decide
from emotional_agent.api.main import app
from emotional_agent.emotion.analyzer import analyze
from emotional_agent.infrastructure.config import Settings, get_settings
from emotional_agent.infrastructure.llm import CompatibleGateway, DemoGateway, ModelError
from emotional_agent.memory.manager import extract, retrieve
from emotional_agent.schemas.models import AgentState, EmotionAnalysis, Memory, RespondRequest
from emotional_agent.state.engine import next_state


class InvalidGateway:
    async def structured(self, system: str, payload: str) -> str:
        return '{"label":"sadness","intensity":4,"confidence":1}'

    async def stream(self, system: str, payload: str) -> AsyncIterator[str]:
        yield "你只能"
        yield "有我"


def request(message: str = "今天有点累", **kwargs: object) -> RespondRequest:
    return RespondRequest.model_validate(
        {
            "request_id": "req_test",
            "turn_id": "turn_test",
            "user_id": "usr_test",
            "conversation_id": "conv_test",
            "message": message,
            **kwargs,
        }
    )


def test_schema_rejects_unbounded_model_output() -> None:
    with pytest.raises(ValidationError):
        EmotionAnalysis(label="sadness", intensity=2, confidence=1)


@pytest.mark.parametrize("text", ["别再问了，我想安静一会", "我不想聊了", "STOP TALKING"])
def test_stop_wins_over_all_other_strategies(text: str) -> None:
    emotion = EmotionAnalysis(label="sadness", intensity=0.9, confidence=0.95, risk="elevated")
    assert decide(text, emotion).action == "respect_space"


def test_negative_emotion_does_not_force_advice() -> None:
    emotion = EmotionAnalysis(label="sadness", intensity=0.7, confidence=0.85)
    assert decide("今天被领导批评了，很难受", emotion).action == "empathize"
    assert decide("怎么办", emotion.model_copy(update={"need": "advice"})).action == "advise"


async def test_invalid_emotion_degrades_without_guessing() -> None:
    emotion = await analyze(InvalidGateway(), "我很累", [])
    assert emotion.label == "unknown" and emotion.confidence == 0


def test_memory_requires_consent_confidence_and_nonexpiry() -> None:
    base = Memory(id="approved", content="我周五有面试", consent_status="approved", confidence=0.95)
    memories = [
        base,
        base.model_copy(update={"id": "pending", "consent_status": "pending"}),
        base.model_copy(
            update={"id": "expired", "expires_at": datetime.now(UTC) - timedelta(days=1)}
        ),
        base.model_copy(update={"id": "uncertain", "confidence": 0.4}),
    ]
    assert [memory.id for memory in retrieve(memories, "还记得我的面试吗")] == ["approved"]


async def test_explicit_demo_memory_stays_candidate() -> None:
    result = await extract(DemoGateway(), "请记住：我喜欢下雨天")
    assert len(result) == 1 and result[0].source_quote == "我喜欢下雨天"
    assert await extract(DemoGateway(), "请记住：我的密码是123456") == []


async def test_without_consent_no_memory_is_used_or_extracted() -> None:
    events = [
        event
        async for event in respond(
            DemoGateway(),
            request(
                "请记住：我周五面试",
                memories=[{"id": "m", "content": "面试", "consent_status": "approved"}],
            ),
            "demo",
        )
    ]
    result = events[-1][1]
    assert result["candidates"] == [] and result["retrieved_memory_ids"] == []
    assert [event for event, _ in events][0] == "message_start"


async def test_stop_never_asks_question_or_extracts_memory() -> None:
    events = [
        event
        async for event in respond(
            DemoGateway(), request("别再问了，我想安静一会", memory_consent=True), "demo"
        )
    ]
    assert events[-1][1]["strategy"]["action"] == "respect_space"
    assert "？" not in events[-1][1]["response"]
    assert events[-1][1]["candidates"] == []


async def test_output_guard_handles_split_tokens() -> None:
    events = [event async for event in respond(InvalidGateway(), request("你好"), "live")]
    assert events[-1][0] == "error"
    assert all(event != "delta" for event, _ in events)


def test_simulated_state_is_bounded_over_many_turns() -> None:
    state = AgentState()
    emotion = EmotionAnalysis(label="joy", intensity=1, confidence=1)
    for _ in range(10000):
        state = next_state(state, emotion)
    assert -1 <= state.valence <= 1 and 0 <= state.arousal <= 1
    assert 0 <= state.familiarity <= 1 and 0.3 <= state.interaction_energy <= 1


def test_state_decays_towards_baseline_without_punishing_absence() -> None:
    old = AgentState(valence=1, arousal=1, updated_at=datetime.now(UTC) - timedelta(days=10))
    state = next_state(old, EmotionAnalysis(label="unknown", intensity=0, confidence=0))
    assert 0.1 < state.valence < 0.2 and 0.2 < state.arousal < 0.3
    assert state.interaction_energy >= 0.3


def test_memory_rejects_ambiguous_timezone() -> None:
    with pytest.raises(ValidationError):
        Memory(id="m", content="面试", expires_at=datetime(2026, 11, 13, 14, 0))


async def test_gateway_retries_only_transient_structured_failures() -> None:
    calls = 0

    def transport(req: httpx.Request) -> httpx.Response:
        nonlocal calls
        calls += 1
        if calls == 1:
            return httpx.Response(503)
        return httpx.Response(200, json={"choices": [{"message": {"content": "{}"}}]})

    settings = Settings(service_token="s" * 32)
    async with httpx.AsyncClient(
        base_url="https://example.invalid/v1/", transport=httpx.MockTransport(transport)
    ) as client:
        gateway = CompatibleGateway(settings, client)
        assert await gateway.structured("system", "data") == "{}"
    assert calls == 2


async def test_gateway_does_not_retry_auth_failure() -> None:
    calls = 0

    def transport(req: httpx.Request) -> httpx.Response:
        nonlocal calls
        calls += 1
        return httpx.Response(401)

    async with httpx.AsyncClient(
        base_url="https://example.invalid/v1/", transport=httpx.MockTransport(transport)
    ) as client:
        with pytest.raises(ModelError):
            await CompatibleGateway(Settings(service_token="s" * 32), client).structured(
                "system", "data"
            )
    assert calls == 1


async def test_incomplete_provider_stream_is_error() -> None:
    transport = httpx.MockTransport(
        lambda req: httpx.Response(
            200, text='data: {"choices":[{"delta":{"content":"hello"}}]}\n\n'
        )
    )
    async with httpx.AsyncClient(
        base_url="https://example.invalid/", transport=transport
    ) as client:
        with pytest.raises(ModelError, match="incomplete_stream"):
            _ = [
                part
                async for part in CompatibleGateway(
                    Settings(service_token="s" * 32), client
                ).stream("", "")
            ]


def test_internal_api_auth_and_sse_schema(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("SERVICE_TOKEN", "test-service-token-" * 3)
    monkeypatch.setenv("LLM_MODE", "demo")
    get_settings.cache_clear()
    with TestClient(app) as client:
        body = request("你好").model_dump(mode="json")
        assert client.post("/internal/v1/agent/respond", json=body).status_code == 401
        response = client.post(
            "/internal/v1/agent/respond",
            json=body,
            headers={"Authorization": "Bearer " + "test-service-token-" * 3},
        )
        assert response.status_code == 200
        assert "event: delta" in response.text and "event: message_end" in response.text
        assert (
            client.post(
                "/internal/v1/agent/respond",
                json={**body, "message": ""},
                headers={"Authorization": "Bearer " + "test-service-token-" * 3},
            ).status_code
            == 422
        )
    get_settings.cache_clear()
