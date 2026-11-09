from emotional_agent.safety.guard import is_risk, is_stop
from emotional_agent.schemas.models import EmotionAnalysis, StrategyDecision


def decide(message: str, emotion: EmotionAnalysis) -> StrategyDecision:
    if is_stop(message) or emotion.need == "space":
        return StrategyDecision(action="respect_space", reason_code="user_boundary")
    if is_risk(message) or emotion.risk == "elevated":
        return StrategyDecision(action="safety_support", reason_code="risk_signal")
    if emotion.confidence < 0.55:
        return StrategyDecision(action="listen", reason_code="uncertain_emotion")
    if emotion.need == "advice":
        return StrategyDecision(action="advise", reason_code="requested_advice")
    if emotion.label in {"sadness", "anger", "anxiety", "fatigue"}:
        return StrategyDecision(
            action="empathize", reason_code="negative_emotion_no_advice_request"
        )
    if emotion.label == "joy":
        return StrategyDecision(action="lighten", reason_code="share_joy")
    return StrategyDecision(action="listen", reason_code="default_listen")
