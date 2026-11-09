"""Generate reviewable OpenAPI contracts from explicit public types and AI schemas."""
import json
import re
from pathlib import Path
from typing import Any

from emotional_agent.api.main import app

ROOT = Path(__file__).resolve().parents[1]
S = {"type": "string"}
B = {"type": "boolean"}
N = {"type": "number", "minimum": 0, "maximum": 1}
DT = {"type": "string", "format": "date-time"}


def obj(properties: dict[str, Any], required: list[str] | None = None) -> dict[str, Any]:
    return {"type": "object", "properties": properties, "required": required if required is not None else list(properties), "additionalProperties": False}


def ref(name: str) -> dict[str, str]:
    return {"$ref": f"#/components/schemas/{name}"}


def arr(name: str) -> dict[str, Any]:
    return {"type": "array", "items": ref(name)}


schemas: dict[str, Any] = {
    "User": obj({"id": S, "email": {"type": "string", "format": "email"}, "memory_consent": B, "created_at": DT}),
    "Credentials": obj({"email": S, "password": {"type": "string", "minLength": 10, "maxLength": 72}, "adult_confirmed": B}, ["email", "password"]),
    "Persona": obj({"name": {"type": "string", "minLength": 1, "maxLength": 24}, "user_name": {"type": "string", "minLength": 1, "maxLength": 24}, "warmth": N, "brevity": N, "humor": N, "version": {"type": "integer", "minimum": 1}}, ["name", "user_name", "warmth", "brevity", "humor"]),
    "Conversation": obj({"id": S, "title": S, "created_at": DT}),
    "Message": obj({"id": S, "turn_id": S, "role": {"enum": ["user", "assistant"]}, "content": S, "created_at": DT}),
    "Memory": obj({"id": S, "content": S, "type": {"enum": ["semantic", "episodic", "boundary"]}, "consent_status": {"enum": ["pending", "approved", "rejected"]}, "confidence": N, "importance": N, "source_message_id": S, "source_quote": S, "expires_at": {"type": ["string", "null"], "format": "date-time"}, "created_at": DT}),
    "MemoryUpdate": obj({"content": {"type": "string", "minLength": 1, "maxLength": 1000}, "consent_status": {"enum": ["pending", "approved", "rejected"]}, "expires_at": {"type": ["string", "null"], "format": "date-time"}}, ["content", "consent_status"]),
    "Preferences": obj({"opt_in": B, "no_contact": B, "quiet_start": {"type": "integer", "minimum": 0, "maximum": 23}, "quiet_end": {"type": "integer", "minimum": 0, "maximum": 23}, "daily_limit": {"type": "integer", "minimum": 1, "maximum": 10}, "timezone": S}),
    "EventCreate": obj({"content": {"type": "string", "minLength": 1, "maxLength": 500}, "due_at": DT, "expires_at": DT, "recurrence": {"enum": ["none", "daily", "weekly"]}, "client_key": {"type": "string", "minLength": 8, "maxLength": 80}}, ["content", "due_at", "recurrence", "client_key"]),
    "ScheduledEvent": obj({"id": S, "content": S, "due_at": DT, "expires_at": DT, "recurrence": {"enum": ["none", "daily", "weekly"]}, "status": {"enum": ["pending", "delivered", "cancelled", "expired"]}, "client_key": S}),
    "Notification": obj({"id": S, "event_id": S, "content": S, "delivered_at": DT}),
    "AgentResult": {"type": "object", "properties": {"turn_id": S, "response": S, "mode": {"enum": ["demo", "live"]}, "strategy": {"type": "object"}, "state": {"type": "object"}, "emotion": {"type": "object"}, "retrieved_memory_ids": {"type": ["array", "null"], "items": S}, "candidates": {"type": ["array", "null"], "items": {"type": "object"}}, "persona_version": {"type": "integer"}, "prompt_version": S, "latency_ms": {"type": "integer"}}, "required": ["turn_id", "response", "mode", "strategy"]},
    "Export": obj({"user": ref("User"), "persona": ref("Persona"), "conversations": arr("Conversation"), "messages": arr("Message"), "memories": arr("Memory"), "notification_preferences": ref("Preferences"), "events": arr("ScheduledEvent"), "notifications": arr("Notification")}),
    "Error": obj({"code": S, "message": S, "data": {"type": "null"}, "request_id": S}),
}

schemas["Persona"]["properties"].update({"id": S,"card_id":S,"relationship":{"enum":["lover","family","friend","custom"]},"description":{"type":"string","maxLength":2000},"greeting":{"type":"string","maxLength":500},"voice_id":{"type":"string","maxLength":100}})
schemas["CharacterCard"]=obj({"id":S,"schema_version":{"const":"companion.character.v1"},"definition":ref("Persona"),"version":{"type":"integer"}},["schema_version","definition"])
schemas["Conversation"]["properties"]["companion_id"]=S
schemas["Memory"]["properties"]["companion_id"]=S
schemas["Export"]["properties"].update({"companions":arr("Persona"),"character_cards":arr("CharacterCard")})

paths: dict[str, Any] = {}


def route(method: str, path: str, summary: str, output: dict[str, Any] | None = None, body: dict[str, Any] | None = None, public: bool = False, status: int = 200) -> None:
    envelope = obj({"code": {"const": "OK"}, "message": S, "data": output or {"type": "null"}, "request_id": S})
    operation: dict[str, Any] = {"summary": summary, "operationId": method.lower() + path.replace("/", "_").replace("{", "").replace("}", "").replace("-", "_"), "responses": {str(status): {"description": "Success", "content": {"application/json": {"schema": envelope}}}}}
    if public:
        operation["security"] = []
    for parameter in re.findall(r"\{([^}]+)\}",path):
        operation.setdefault("parameters",[]).append({"name":parameter,"in":"path","required":True,"schema":S})
    if body:
        operation["requestBody"] = {"required": True, "content": {"application/json": {"schema": body}}}
    for code in (400, 401, 404, 409, 429, 500, 503):
        operation["responses"][str(code)] = {"description": "Business error", "content": {"application/json": {"schema": ref("Error")}}}
    paths.setdefault(path, {})[method.lower()] = operation


route("GET", "/healthz", "Liveness", obj({"status": S}), public=True)
route("GET", "/readyz", "Database and Redis readiness", obj({"status": S}), public=True)
route("POST", "/api/v1/auth/register", "Register adult account and set HttpOnly session cookie", ref("User"), ref("Credentials"), True, 201)
route("POST", "/api/v1/auth/login", "Login and set HttpOnly session cookie", ref("User"), ref("Credentials"), True)
route("POST", "/api/v1/auth/logout", "Clear browser session")
route("GET", "/api/v1/me", "Current account", ref("User"))
route("GET", "/api/v1/status", "Runtime model and audio capabilities without secrets", obj({"mode": {"enum": ["demo", "live"]}, "version": S,"asr_enabled":B,"tts_enabled":B}))
route("PATCH", "/api/v1/me/preferences", "Enable collection and recall; individual confirmation still required", ref("User"), obj({"memory_consent": B}))
route("GET", "/api/v1/me/export", "Export own data", ref("Export"))
route("DELETE", "/api/v1/me", "Permanently delete account and associated rows", body=obj({"confirmation": {"const": "DELETE"}}))
route("GET", "/api/v1/persona", "Get stable persona", ref("Persona"))
route("PUT", "/api/v1/persona", "Update persona; version incremented by server", ref("Persona"), ref("Persona"))
route("GET", "/api/v1/conversations", "List own conversations", arr("Conversation"))
route("POST", "/api/v1/conversations", "Create conversation permanently bound to a companion", ref("Conversation"), obj({"title": {"type": "string", "maxLength": 80},"companion_id":S}, []), status=201)
route("GET", "/api/v1/conversations/{id}/messages", "Read own timeline", arr("Message"))
route("POST", "/api/v1/conversations/{id}/messages", "Send or retry idempotent turn; Accept text/event-stream for SSE", ref("AgentResult"), obj({"content": {"type": "string", "minLength": 1, "maxLength": 8000}, "turn_id": {"type": "string", "minLength": 8, "maxLength": 80}}))
send = paths["/api/v1/conversations/{id}/messages"]["post"]
send["responses"]["200"]["content"]["text/event-stream"] = {"schema": S}
route("GET", "/api/v1/conversations/{id}/stream", "Replay completed turn snapshot; message_end contains complete text", ref("AgentResult"))
replay = paths["/api/v1/conversations/{id}/stream"]["get"]
replay["parameters"] += [{"name": "turn_id", "in": "query", "required": True, "schema": S}, {"name": "Last-Event-ID", "in": "header", "schema": {"type": "integer", "minimum": 0, "maximum": 3}}]
replay["responses"]["200"]["content"] = {"text/event-stream": {"schema": S}}
route("GET", "/api/v1/memories", "List own pending and confirmed memories", arr("Memory"))
route("PATCH", "/api/v1/memories/{id}", "Correct or confirm memory; rejecting deletes it", {"anyOf": [ref("Memory"), {"type": "null"}]}, ref("MemoryUpdate"))
route("DELETE", "/api/v1/memories/{id}", "Delete own memory and vector atomically")
route("DELETE", "/api/v1/memories", "Delete all own memories")
route("POST", "/api/v1/feedback", "Rate owned assistant message", body=obj({"message_id": S, "rating": {"enum": [-1, 1]}, "tag": {"type": "string", "maxLength": 80}}, ["message_id", "rating"]), status=201)
route("GET", "/api/v1/notification-preferences", "Read explicit notification consent", ref("Preferences"))
route("PUT", "/api/v1/notification-preferences", "Set consent, quiet hours and quota", ref("Preferences"), ref("Preferences"))
route("GET", "/api/v1/events", "List own reminder tasks", arr("ScheduledEvent"))
route("POST", "/api/v1/events", "Create authorized reminder with idempotency key", ref("ScheduledEvent"), ref("EventCreate"), status=201)
route("DELETE", "/api/v1/events/{id}", "Revoke reminder")
route("GET", "/api/v1/notifications", "Read authorized in-app notifications", arr("Notification"))
route("GET","/api/v1/companions","List independent owned AI companions",arr("Persona"))
route("POST","/api/v1/companions","Create an independent companion from owned card",ref("Persona"),obj({"card_id":S,"name":{"type":"string","minLength":1,"maxLength":24}}),status=201)
route("GET","/api/v1/companions/{id}","Read owned companion",ref("Persona"))
route("PUT","/api/v1/companions/{id}","Edit only this companion; original template unchanged",ref("Persona"),ref("Persona"))
route("POST","/api/v1/companions/{id}/activate","Select companion for new UI interactions",ref("Persona"))
route("GET","/api/v1/character-cards","List reusable character templates",arr("CharacterCard"))
route("POST","/api/v1/character-cards","Import or save strict versioned JSON character template",ref("CharacterCard"),ref("CharacterCard"),status=201)
route("POST","/api/v1/audio/transcriptions","Transcribe one <=10MB audio recording without storing audio",obj({"text":S,"duration_seconds":{"type":["number","null"]}}),obj({"companion_id":S,"audio_base64":{"type":"string","maxLength":14000000},"format":{"enum":["wav","mp3","webm","ogg","opus","flac","m4a","mp4","aac"]}}))
route("POST","/api/v1/conversations/{id}/messages/{message}/audio","Synthesize an owned assistant reply using its companion voice")
paths['/api/v1/conversations/{id}/messages/{message}/audio']['post']['responses']['200']['content']={'audio/mpeg':{'schema':{'type':'string','format':'binary'}}}
route("GET","/api/v1/integrations/tools","Discover operator-approved MCP tools for user",{"type":"array","items":{"type":"object"}})
paths['/api/v1/integrations/tools']['get']['parameters']=[{'name':'companion_id','in':'query','required':True,'schema':S}]
route("POST","/api/v1/integrations/tools/invoke","Explicitly call an authorized read-only MCP tool",{"type":"object"},obj({"companion_id":S,"tool_id":S,"arguments":{"type":"object"}}))
for method in ('get','delete'):
    paths['/api/v1/memories'][method]['parameters']=[{'name':'companion_id','in':'query','schema':S,'description':'GET defaults to active companion; DELETE without this deletes all account memories.'}]

document = {"openapi": "3.1.0", "info": {"title": "知夏 Companion API", "version": "0.1.0", "description": "Cookie mutations require exact configured Origin. Bearer auth is supported for trusted API clients. SSE logical IDs: start=1, delta=2, end=3; replay is a full-turn snapshot, not individual token replay."}, "security": [{"sessionCookie": []}, {"bearerAuth": []}], "paths": paths, "components": {"securitySchemes": {"sessionCookie": {"type": "apiKey", "in": "cookie", "name": "companion_session"}, "bearerAuth": {"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}}, "schemas": schemas}}
internal = app.openapi()
internal["components"]["securitySchemes"] = {"serviceToken": {"type": "http", "scheme": "bearer"}}
for path, operations in internal['paths'].items():
    if path.startswith('/internal/'):
        for operation in operations.values(): operation['security']=[{'serviceToken':[]}]
internal["paths"]["/internal/v1/agent/respond"]["post"]["security"] = [{"serviceToken": []}]
internal["paths"]["/internal/v1/agent/respond"]["post"]["responses"]["200"]["content"] = {"text/event-stream": {"schema": S}}
internal['paths']['/internal/v1/audio/speech']['post']['responses']['200']['content']={'audio/mpeg':{'schema':{'type':'string','format':'binary'}}}
for name, spec in (("openapi.json", document), ("ai.openapi.json", internal)):
    (ROOT / "packages" / "contracts" / name).write_text(json.dumps(spec, ensure_ascii=False, indent=2)+"\n", encoding="utf-8")
print("Generated public and internal OpenAPI contracts")
