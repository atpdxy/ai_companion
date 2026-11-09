from typing import Literal

from pydantic import AwareDatetime, BaseModel, ConfigDict, Field


class Schema(BaseModel):
    model_config = ConfigDict(extra="forbid")


class ExecutionBudgets(Schema):
    emotion_seconds: float = Field(default=12, gt=0, le=120)
    generation_seconds: float = Field(default=65, gt=0, le=120)
    memory_seconds: float = Field(default=10, gt=0, le=120)


class EmotionAnalysis(Schema):
    label: Literal["neutral", "joy", "sadness", "anger", "anxiety", "fatigue", "unknown"]
    intensity: float = Field(ge=0, le=1)
    confidence: float = Field(ge=0, le=1)
    need: Literal["listen", "advice", "space", "uncertain"] = "uncertain"
    risk: Literal["normal", "elevated"] = "normal"


class Persona(Schema):
    id: str = ""
    card_id: str = ""
    relationship: Literal["lover", "family", "friend", "custom"] = "lover"
    description: str = Field(default="", max_length=2000)
    greeting: str = Field(default="", max_length=500)
    voice_id: str = Field(default="", max_length=100)
    name: str = Field(default="知夏", min_length=1, max_length=24)
    user_name: str = Field(default="你", min_length=1, max_length=24)
    warmth: float = Field(default=0.85, ge=0, le=1)
    brevity: float = Field(default=0.6, ge=0, le=1)
    humor: float = Field(default=0.35, ge=0, le=1)
    version: int = Field(default=1, ge=1)


class AgentState(Schema):
    updated_at: AwareDatetime | None = None
    valence: float = Field(default=0.15, ge=-1, le=1)
    arousal: float = Field(default=0.25, ge=0, le=1)
    familiarity: float = Field(default=0, ge=0, le=1)
    interaction_energy: float = Field(default=0.8, ge=0, le=1)


class ChatMessage(Schema):
    role: Literal["user", "assistant"]
    content: str = Field(max_length=12000)


class Memory(Schema):
    id: str
    content: str = Field(max_length=1000)
    type: Literal["semantic", "episodic", "boundary"] = "semantic"
    consent_status: Literal["pending", "approved", "rejected"] = "pending"
    confidence: float = Field(default=1, ge=0, le=1)
    importance: float = Field(default=0.5, ge=0, le=1)
    expires_at: AwareDatetime | None = None


class MemoryCandidate(Schema):
    content: str = Field(min_length=1, max_length=1000)
    type: Literal["semantic", "episodic", "boundary"]
    confidence: float = Field(ge=0, le=1)
    source_quote: str = Field(min_length=1, max_length=1000)


class CandidateList(Schema):
    candidates: list[MemoryCandidate] = Field(default_factory=list, max_length=3)


class StrategyDecision(Schema):
    action: Literal[
        "listen", "empathize", "clarify", "advise", "lighten", "respect_space", "safety_support"
    ]
    reason_code: str
    prohibited_actions: list[str] = Field(default_factory=lambda: ["diagnose", "induce_dependency"])


class RespondRequest(Schema):
    request_id: str = Field(min_length=1, max_length=80)
    turn_id: str = Field(min_length=1, max_length=80)
    user_id: str = Field(min_length=1, max_length=80)
    conversation_id: str = Field(min_length=1, max_length=80)
    message: str = Field(min_length=1, max_length=8000)
    context: list[ChatMessage] = Field(default_factory=list, max_length=20)
    memories: list[Memory] = Field(default_factory=list, max_length=50)
    persona: Persona = Field(default_factory=Persona)
    state: AgentState = Field(default_factory=AgentState)
    memory_consent: bool = False
