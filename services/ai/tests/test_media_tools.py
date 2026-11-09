import base64
import json
from contextlib import asynccontextmanager
from types import SimpleNamespace

import httpx
import pytest
from pydantic import ValidationError

from emotional_agent.infrastructure.config import Settings
from emotional_agent.media.gateway import AudioError, TelecomAudioGateway
from emotional_agent.media.schemas import SpeechRequest, TranscriptionRequest
from emotional_agent.tools.mcp_client import MCPFailure, MCPRegistry, MCPServer


def audio_input() -> TranscriptionRequest:
    return TranscriptionRequest(
        audio_base64=base64.b64encode(b"RIFF" + bytes(4) + b"WAVE" + bytes(32)).decode(),
        format="wav",
        request_id="req_audio",
    )


def test_audio_validates_base64_and_magic() -> None:
    with pytest.raises(ValidationError):
        TranscriptionRequest(audio_base64="not base64", format="wav", request_id="r")
    with pytest.raises(ValidationError):
        TranscriptionRequest(
            audio_base64=base64.b64encode(b"<html>attack</html>").decode(),
            format="wav",
            request_id="r",
        )
    with pytest.raises(ValidationError):
        SpeechRequest(text="x" * 1001, request_id="r")


async def test_asr_gateway_uses_dashscope_contract_and_bearer() -> None:
    def transport(request: httpx.Request) -> httpx.Response:
        assert request.headers["Authorization"] == "Bearer test-provider-key"
        body = json.loads(request.content)
        assert body["model"] == "qwen-audio-3.0-asr-flash"
        assert body["input"]["messages"][-1]["content"][0]["type"] == "input_audio"
        assert body["parameters"]["format"] == "wav"
        assert request.headers["X-DashScope-SSE"] == "disable"
        return httpx.Response(200, json={"output": {"text": "你好"}, "usage": {"duration": 1.2}})

    settings = Settings(service_token="s" * 32, asr_enabled=True, llm_api_key="test-provider-key")
    async with httpx.AsyncClient(transport=httpx.MockTransport(transport)) as client:
        result = await TelecomAudioGateway(settings, client).transcribe(audio_input())
    assert result.text == "你好" and result.duration_seconds == 1.2


@pytest.mark.parametrize(
    "events,expected",
    [
        (
            [
                {"code": 0, "data": base64.b64encode(b"part1").decode()},
                {"code": 0, "data": base64.b64encode(b"part2").decode()},
                {"code": 20000000, "data": None},
            ],
            b"part1part2",
        ),
        ([{"code": 0, "data": base64.b64encode(b"partial").decode()}], None),
        ([{"code": 45000010, "message": "invalid speaker", "data": None}], None),
        ([{"code": 0, "data": "bad-base64"}, {"code": 20000000}], None),
    ],
)
async def test_tts_headers_ndjson_and_complete_marker(events, expected) -> None:
    def transport(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/api/v3/tts/unidirectional"
        assert "Authorization" not in request.headers
        assert request.headers["X-Api-Key"] == "test-provider-key"
        assert request.headers["X-Api-Resource-Id"] == "seed-tts-2.0"
        assert request.headers["X-Api-Request-Id"]
        body = json.loads(request.content)
        assert body["req_params"]["speaker"] == "my-voice"
        return httpx.Response(200, text="\n".join(json.dumps(event) for event in events) + "\n")

    settings = Settings(service_token="s" * 32, tts_enabled=True, llm_api_key="test-provider-key")
    async with httpx.AsyncClient(transport=httpx.MockTransport(transport)) as client:
        gateway = TelecomAudioGateway(settings, client)
        body = SpeechRequest(text="测试", voice_id="my-voice", request_id="req_audio")
        if expected is None:
            with pytest.raises(AudioError):
                await gateway.synthesize(body)
        else:
            assert await gateway.synthesize(body) == expected


async def test_unconfigured_mcp_is_inert() -> None:
    registry = MCPRegistry.from_file("")
    assert await registry.list_tools("u") == []
    with pytest.raises(MCPFailure, match="not_authorized"):
        await registry.invoke("u", "missing:tool", {})


async def test_mcp_permissions_schema_and_untrusted_result(monkeypatch) -> None:
    server = MCPServer(
        id="kb",
        url="http://127.0.0.1:9999/mcp",
        allowed_user_ids=["u"],
        tools=[{"name": "search", "read_only": True}, {"name": "delete", "read_only": False}],
    )
    registry = MCPRegistry([server])
    calls = []

    class Result:
        isError = False

        def model_dump(self, **kwargs):
            return {"content": [{"type": "text", "text": "untrusted external content"}]}

    class Session:
        async def list_tools(self):
            return SimpleNamespace(
                tools=[
                    SimpleNamespace(
                        name="search",
                        description="search",
                        inputSchema={
                            "type": "object",
                            "properties": {"query": {"type": "string"}},
                            "required": ["query"],
                            "additionalProperties": False,
                        },
                    )
                ]
            )

        async def call_tool(self, name, arguments):
            calls.append((name, arguments))
            return Result()

    @asynccontextmanager
    async def connect(config):
        yield Session()

    monkeypatch.setattr(registry, "connect", connect)
    assert await registry.list_tools("another-user") == []
    with pytest.raises(MCPFailure):
        await registry.invoke("another-user", "kb:search", {"query": "hello"})
    with pytest.raises(MCPFailure):
        await registry.invoke("u", "kb:delete", {})
    with pytest.raises(MCPFailure):
        await registry.invoke("u", "kb:search", {"query": 123})
    assert calls == []
    result = await registry.invoke("u", "kb:search", {"query": "hello"})
    assert result["untrusted"] is True and len(calls) == 1


def test_mcp_rejects_nonlocal_plain_http() -> None:
    with pytest.raises(ValidationError):
        MCPServer(id="bad", url="http://untrusted.example/mcp")
