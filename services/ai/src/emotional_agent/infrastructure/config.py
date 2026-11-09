from functools import lru_cache
from typing import Literal

from pydantic import Field, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore", hide_input_in_errors=True)
    service_token: str = Field(min_length=32)
    llm_mode: Literal["demo", "live"] = "demo"
    llm_base_url: str = ""
    llm_api_key: str = ""
    llm_model: str = ""
    llm_timeout_seconds: float = Field(default=40, gt=0, le=120)
    llm_max_tokens: int = Field(default=700, ge=64, le=4096)
    llm_temperature: float = Field(default=0.7, ge=0, le=2)
    llm_max_retries: int = Field(default=2, ge=0, le=3)
    emotion_timeout_seconds: float = Field(default=12, gt=0, le=30)
    generation_timeout_seconds: float = Field(default=65, gt=0, le=90)
    memory_timeout_seconds: float = Field(default=10, gt=0, le=15)
    asr_enabled: bool = False
    asr_url: str = (
        "https://aigw.telecomjs.com/api/v1/services/aigc/multimodal-generation/generation"
    )
    asr_model: str = "qwen-audio-3.0-asr-flash"
    asr_api_key: str = ""
    tts_enabled: bool = False
    tts_url: str = "https://aigw.telecomjs.com/api/v3/tts/unidirectional"
    tts_model: str = "seed-tts-2.0"
    tts_speaker: str = "zh_female_vv_uranus_bigtts"
    tts_api_key: str = ""
    audio_timeout_seconds: float = Field(default=75, gt=0, le=90)
    mcp_servers_file: str = ""
    mcp_timeout_seconds: float = Field(default=15, gt=0, le=30)

    @model_validator(mode="after")
    def validate_live(self) -> "Settings":
        if self.asr_enabled and not (self.asr_api_key or self.llm_api_key):
            raise ValueError("ASR enabled without API key")
        if self.tts_enabled and not (self.tts_api_key or self.llm_api_key):
            raise ValueError("TTS enabled without API key")
        if self.llm_mode == "live" and not all(
            (self.llm_base_url, self.llm_api_key, self.llm_model)
        ):
            raise ValueError("live 模式需要 LLM_BASE_URL、LLM_API_KEY、LLM_MODEL")
        return self


@lru_cache
def get_settings() -> Settings:
    return Settings()  # type: ignore[call-arg]
