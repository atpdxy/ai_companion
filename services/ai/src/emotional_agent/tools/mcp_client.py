import asyncio
import json
import logging
import os
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

import httpx
from jsonschema import Draft202012Validator
from jsonschema.exceptions import SchemaError, ValidationError
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from mcp.shared.exceptions import McpError
from pydantic import BaseModel, ConfigDict, Field, model_validator
from referencing import Registry
from referencing.exceptions import Unresolvable


class MCPFailure(Exception):
    """Sanitized integration failure."""


class AllowedTool(BaseModel):
    model_config = ConfigDict(extra="forbid")
    name: str = Field(min_length=1, max_length=100)
    read_only: bool = False


class MCPServer(BaseModel):
    model_config = ConfigDict(extra="forbid")
    id: str = Field(pattern=r"^[a-zA-Z0-9_-]{1,40}$")
    url: str
    allowed_user_ids: list[str] = Field(default_factory=list, max_length=100)
    tools: list[AllowedTool] = Field(default_factory=list, max_length=32)
    token_env: str = ""

    @model_validator(mode="after")
    def validate_url(self) -> "MCPServer":
        parsed = urlparse(self.url)
        if parsed.scheme != "https" and not (
            parsed.scheme == "http" and parsed.hostname in {"127.0.0.1", "localhost", "::1"}
        ):
            raise ValueError("MCP requires HTTPS or loopback HTTP")
        if parsed.username or parsed.password or not parsed.hostname:
            raise ValueError("invalid MCP URL")
        return self


class ToolDescriptor(BaseModel):
    id: str
    description: str
    input_schema: dict[str, Any]


class MCPRegistry:
    def __init__(self, servers: list[MCPServer], timeout: float = 15) -> None:
        self.servers = servers
        self.timeout = timeout
        # Third-party diagnostics must not log private tool arguments or result bodies.
        sdk_logger = logging.getLogger("mcp")
        sdk_logger.addHandler(logging.NullHandler())
        sdk_logger.propagate = False

    @classmethod
    def from_file(cls, path: str, timeout: float = 15) -> "MCPRegistry":
        if not path:
            return cls([], timeout)
        raw = Path(path).read_text(encoding="utf-8")
        if len(raw) > 100_000:
            raise ValueError("MCP configuration too large")
        data = json.loads(raw)
        if not isinstance(data, list) or len(data) > 16:
            raise ValueError("MCP configuration must contain up to 16 servers")
        servers = [MCPServer.model_validate(item) for item in data]
        if len({server.id for server in servers}) != len(servers):
            raise ValueError("duplicate MCP server id")
        return cls(servers, timeout)

    @asynccontextmanager
    async def connect(self, server: MCPServer) -> AsyncIterator[ClientSession]:
        headers = None
        if server.token_env:
            token = os.environ.get(server.token_env)
            if not token:
                raise MCPFailure("missing_mcp_credentials")
            headers = {"Authorization": f"Bearer {token}"}
        async with streamablehttp_client(
            server.url, headers=headers, timeout=self.timeout, sse_read_timeout=self.timeout
        ) as (read, write, _):
            async with ClientSession(read, write) as session:
                await session.initialize()
                yield session

    async def list_tools(self, user_id: str) -> list[ToolDescriptor]:
        descriptors: list[ToolDescriptor] = []
        for server in self.servers:
            if user_id not in server.allowed_user_ids:
                continue
            allowed = {tool.name for tool in server.tools if tool.read_only}
            if not allowed:
                continue
            try:
                async with asyncio.timeout(self.timeout), self.connect(server) as session:
                    result = await session.list_tools()
                    for tool in result.tools:
                        if tool.name in allowed:
                            if len(json.dumps(tool.inputSchema)) > 16_000:
                                raise MCPFailure("tool_schema_too_large")
                            Draft202012Validator.check_schema(tool.inputSchema)
                            descriptors.append(
                                ToolDescriptor(
                                    id=f"{server.id}:{tool.name}",
                                    description=(tool.description or "")[:1000],
                                    input_schema=tool.inputSchema,
                                )
                            )
            except (
                McpError,
                httpx.HTTPError,
                ExceptionGroup,
                OSError,
                TimeoutError,
                SchemaError,
            ) as error:
                raise MCPFailure("mcp_discovery_failed") from error
        return descriptors

    async def invoke(self, user_id: str, tool_id: str, arguments: dict[str, Any]) -> dict[str, Any]:
        if len(json.dumps(arguments)) > 16_000:
            raise MCPFailure("tool_arguments_too_large")
        server_id, separator, name = tool_id.partition(":")
        server = next((item for item in self.servers if item.id == server_id), None)
        if not separator or server is None or user_id not in server.allowed_user_ids:
            raise MCPFailure("tool_not_authorized")
        if not any(tool.name == name and tool.read_only for tool in server.tools):
            raise MCPFailure("tool_not_authorized")
        try:
            async with asyncio.timeout(self.timeout), self.connect(server) as session:
                listing = await session.list_tools()
                tool = next((item for item in listing.tools if item.name == name), None)
                if tool is None:
                    raise MCPFailure("tool_not_found")
                Draft202012Validator.check_schema(tool.inputSchema)
                if len(json.dumps(tool.inputSchema)) > 16_000:
                    raise MCPFailure("tool_schema_too_large")
                # Empty registry refuses remote $ref retrieval and network fetches.
                Draft202012Validator(tool.inputSchema, registry=Registry()).validate(arguments)
                result = await session.call_tool(name, arguments=arguments)
                if result.isError:
                    raise MCPFailure("tool_execution_failed")
                # Never execute content or feed it into the system instruction channel.
                output = result.model_dump(mode="json")
                if len(json.dumps(output)) > 32_000:
                    raise MCPFailure("tool_result_too_large")
                return {"untrusted": True, "tool_id": tool_id, "result": output}
        except (
            McpError,
            httpx.HTTPError,
            ExceptionGroup,
            OSError,
            TimeoutError,
            ValidationError,
            SchemaError,
            Unresolvable,
        ) as error:
            raise MCPFailure("tool_call_failed") from error
