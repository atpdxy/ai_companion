from contextlib import asynccontextmanager
from types import SimpleNamespace

import pytest

from emotional_agent.tools.mcp_client import MCPFailure, MCPRegistry, MCPServer


async def test_mcp_schema_cannot_fetch_external_references(monkeypatch) -> None:
    server = MCPServer(
        id="kb",
        url="http://127.0.0.1:9999/mcp",
        allowed_user_ids=["u"],
        tools=[{"name": "search", "read_only": True}],
    )
    registry = MCPRegistry([server])

    def reject_network(*args, **kwargs):
        raise AssertionError("JSON Schema must not fetch remote references")

    monkeypatch.setattr("urllib.request.urlopen", reject_network)

    class Session:
        async def list_tools(self):
            return SimpleNamespace(
                tools=[
                    SimpleNamespace(
                        name="search",
                        inputSchema={
                            "type": "object",
                            "properties": {
                                "query": {"$ref": "https://must-not-fetch.invalid/schema"}
                            },
                        },
                    )
                ]
            )

        async def call_tool(self, *args, **kwargs):
            raise AssertionError("invalid arguments reached tool execution")

    @asynccontextmanager
    async def connect(config):
        yield Session()

    monkeypatch.setattr(registry, "connect", connect)
    with pytest.raises(MCPFailure):
        await registry.invoke("u", "kb:search", {"query": "test"})
