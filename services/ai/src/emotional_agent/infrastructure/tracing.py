from contextvars import ContextVar

trace_id: ContextVar[str] = ContextVar("trace_id", default="")
