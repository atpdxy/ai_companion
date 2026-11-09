import asyncio
import json
import logging
from collections.abc import AsyncIterator
from typing import Any, Protocol

import httpx

from emotional_agent.infrastructure.config import Settings

logger = logging.getLogger(__name__)


class ModelError(Exception):
    """Upstream failure; never expose provider response bodies to clients."""


class ModelGateway(Protocol):
    async def structured(self, system: str, payload: str) -> str: ...

    def stream(self, system: str, payload: str) -> AsyncIterator[str]: ...


class CompatibleGateway:
    def __init__(self, settings: Settings, client: httpx.AsyncClient) -> None:
        self.settings = settings
        self.client = client

    def _body(self, system: str, payload: str, stream: bool) -> dict[str, Any]:
        return {
            "model": self.settings.llm_model,
            "messages": [
                {"role": "system", "content": system},
                {"role": "user", "content": payload},
            ],
            "temperature": self.settings.llm_temperature,
            "max_tokens": self.settings.llm_max_tokens,
            "stream": stream,
        }

    async def structured(self, system: str, payload: str) -> str:
        body = self._body(system, payload, False)
        body["response_format"] = {"type": "json_object"}
        for attempt in range(self.settings.llm_max_retries + 1):
            try:
                response = await self.client.post("chat/completions", json=body)
                if response.status_code == 429 or response.status_code >= 500:
                    if attempt < self.settings.llm_max_retries:
                        await asyncio.sleep(0.25 * 2**attempt)
                        continue
                response.raise_for_status()
                result = response.json()["choices"][0]["message"]["content"]
                if not isinstance(result, str):
                    raise ModelError("invalid_content")
                return result
            except (httpx.TimeoutException, httpx.NetworkError) as error:
                if attempt == self.settings.llm_max_retries:
                    raise ModelError("provider_unavailable") from error
                await asyncio.sleep(0.25 * 2**attempt)
            except (httpx.HTTPStatusError, KeyError, ValueError, IndexError) as error:
                raise ModelError("provider_rejected_or_invalid") from error
        raise ModelError("retry_exhausted")

    async def stream(self, system: str, payload: str) -> AsyncIterator[str]:
        # A partially emitted response must never be retried implicitly.
        try:
            async with self.client.stream(
                "POST", "chat/completions", json=self._body(system, payload, True)
            ) as response:
                response.raise_for_status()
                completed = False
                async for line in response.aiter_lines():
                    if not line.startswith("data:"):
                        continue
                    raw = line[5:].strip()
                    if raw == "[DONE]":
                        completed = True
                        break
                    event = json.loads(raw)
                    choices = event.get("choices", [])
                    if not choices:
                        continue
                    choice = choices[0]
                    if choice.get("finish_reason") == "length":
                        raise ModelError("token_budget_exceeded")
                    token = choice.get("delta", {}).get("content")
                    if token is not None and not isinstance(token, str):
                        raise ModelError("invalid_delta")
                    if token:
                        yield token
                if not completed:
                    raise ModelError("incomplete_stream")
        except (httpx.HTTPError, ValueError, KeyError, IndexError) as error:
            raise ModelError("stream_failed") from error


class DemoGateway:
    async def structured(self, system: str, payload: str) -> str:
        if "emotion.v1" in system:
            return json.dumps({"label": "unknown", "intensity": 0, "confidence": 0})
        message = json.loads(payload).get("message", "")
        for prefix in ("请记住：", "记住：", "请记住:", "记住:"):
            if message.startswith(prefix) and message[len(prefix) :].strip():
                quote = message[len(prefix) :].strip()
                return json.dumps(
                    {
                        "candidates": [
                            {
                                "content": quote,
                                "type": "semantic",
                                "confidence": 1,
                                "source_quote": quote,
                            }
                        ]
                    },
                    ensure_ascii=False,
                )
        return '{"candidates":[]}'

    async def stream(self, system: str, payload: str) -> AsyncIterator[str]:
        context = json.loads(payload)
        name = context["persona"]["user_name"]
        memories = context["memories"]
        if memories:
            reply = (
                f"{name}，我记得你之前提过：{memories[0]['content']}。"
                "愿意的话，我们可以从这里继续聊。"
            )
        else:
            reply = f"{name}，我在听。你可以按自己的节奏说，也可以先在这里歇一会儿。"
        for offset in range(0, len(reply), 4):
            yield reply[offset : offset + 4]
            await asyncio.sleep(0.015)
