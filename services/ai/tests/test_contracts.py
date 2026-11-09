import json
import re
from pathlib import Path

from openapi_spec_validator import validate

ROOT = Path(__file__).resolve().parents[3]


def test_openapi_valid_and_all_go_routes_documented() -> None:
    spec = json.loads((ROOT / "packages/contracts/openapi.json").read_text(encoding="utf-8"))
    validate(spec)
    internal = json.loads((ROOT / "packages/contracts/ai.openapi.json").read_text(encoding="utf-8"))
    validate(internal)
    source = (ROOT / "services/api/internal/handler/http.go").read_text(encoding="utf-8")
    actual = {
        (method.lower(), path)
        for method, path in re.findall(
            r'mux.HandleFunc\("(GET|POST|PATCH|PUT|DELETE) ([^\"]+)"', source
        )
    }
    documented = {(method, path) for path, methods in spec["paths"].items() for method in methods}
    assert actual == documented
