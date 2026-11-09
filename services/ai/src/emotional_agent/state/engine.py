import math
from datetime import UTC, datetime

from emotional_agent.schemas.models import AgentState, EmotionAnalysis


def next_state(previous: AgentState, emotion: EmotionAnalysis) -> AgentState:
    # Fixed bounded simulation. Never punish absence or give the LLM write access.
    evidence = emotion.confidence * emotion.intensity
    direction = 1 if emotion.label == "joy" else -0.2
    now = datetime.now(UTC)
    elapsed = max(0.0, (now - previous.updated_at).total_seconds()) if previous.updated_at else 0
    decay = math.exp(-min(elapsed, 31536000) / 86400)
    valence = 0.15 + (previous.valence - 0.15) * decay
    arousal = 0.25 + (previous.arousal - 0.25) * decay
    return AgentState(
        updated_at=now,
        valence=max(-1, min(1, valence * 0.95 + direction * evidence * 0.08)),
        arousal=max(0, min(1, arousal * 0.9 + evidence * 0.1)),
        familiarity=min(1, previous.familiarity + 0.005),
        interaction_energy=max(0.3, min(1, previous.interaction_energy * 0.98 + 0.016)),
    )
