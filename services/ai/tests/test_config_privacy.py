import pytest
from pydantic import ValidationError

from emotional_agent.infrastructure.config import Settings


def test_invalid_configuration_does_not_print_credentials() -> None:
    secret = "test-credential-never-log"
    service_token = "s" * 32
    with pytest.raises(ValidationError) as captured:
        Settings(
            service_token=service_token,
            llm_mode="live",
            llm_api_key=secret,
            llm_base_url="",
            llm_model="",
        )
    assert secret not in str(captured.value)
    assert service_token not in str(captured.value)
