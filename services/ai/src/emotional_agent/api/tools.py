import logging
from typing import Annotated, Any

from fastapi import APIRouter, Header, HTTPException, Query, Request
from pydantic import Field

from emotional_agent.api.audio import authorize
from emotional_agent.schemas.models import Schema
from emotional_agent.tools.mcp_client import MCPFailure, MCPRegistry, ToolDescriptor

router = APIRouter(prefix="/internal/v1/tools")
logger = logging.getLogger(__name__)


class ToolCall(Schema):
    user_id: str = Field(min_length=1, max_length=80)
    tool_id: str = Field(min_length=1, max_length=150)
    request_id: str = Field(min_length=1, max_length=80)
    arguments: dict[str, Any]


@router.get("")
async def tools(
    request: Request,
    user_id: Annotated[str, Query(min_length=1, max_length=80)],
    authorization: Annotated[str, Header()] = "",
) -> list[ToolDescriptor]:
    authorize(authorization)
    registry: MCPRegistry = request.app.state.mcp_registry
    try:
        return await registry.list_tools(user_id)
    except MCPFailure as error:
        raise HTTPException(503, "mcp_unavailable") from error


@router.post("/invoke")
async def invoke(
    body: ToolCall, request: Request, authorization: Annotated[str, Header()] = ""
) -> dict[str, Any]:
    authorize(authorization)
    registry: MCPRegistry = request.app.state.mcp_registry
    try:
        result = await registry.invoke(body.user_id, body.tool_id, body.arguments)
        logger.info("tool_invoked", extra={"event": "tool_invoked", "trace_id": body.request_id})
        return result
    except MCPFailure as error:
        logger.warning(
            "tool_call_failed", extra={"event": "tool_call_failed", "trace_id": body.request_id}
        )
        raise HTTPException(400, "tool_denied_or_failed") from error
