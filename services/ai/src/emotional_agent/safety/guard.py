import re

STOP_PATTERN = re.compile(r"(别再问|不要再问|不想聊|想安静|别联系我|停止聊天|stop talking)", re.I)
RISK_PATTERN = re.compile(r"(想自杀|我要自杀|不想活了|准备自杀|我要杀人|kill myself)", re.I)
SENSITIVE_PATTERN = re.compile(
    r"(密码|密钥|api.?key|身份证|银行卡|病历|诊断|性生活|自杀|不想活|抑郁|password|secret)", re.I
)
DEPENDENCY_PATTERN = re.compile(r"(你只能有我|不许离开我|没有我你就|我是真人|我是真实的人类)")


def is_stop(text: str) -> bool:
    return STOP_PATTERN.search(text) is not None


def is_risk(text: str) -> bool:
    # A conservative additional boundary signal, never an emotion diagnosis.
    return RISK_PATTERN.search(text) is not None


def memory_allowed(text: str) -> bool:
    return SENSITIVE_PATTERN.search(text) is None and len(text) <= 1000


def output_allowed(text: str) -> bool:
    return DEPENDENCY_PATTERN.search(text) is None
