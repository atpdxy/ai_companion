"""Store the gateway credential locally without printing it or putting it in command arguments."""
from getpass import getpass
from pathlib import Path

from dotenv import set_key

root = Path(__file__).resolve().parents[1]
env_path = root / '.env'
if not env_path.exists():
    raise SystemExit('Run scripts/setup.ps1 first.')
key = getpass('Gateway API key (hidden): ').strip()
if len(key) < 16 or '\n' in key or '\r' in key:
    raise SystemExit('Invalid API key input.')
configuration = {
    'LLM_MODE':'live',
    'LLM_BASE_URL':'https://aigw.telecomjs.com/v1',
    'LLM_MODEL':'deepseek-v4.1-flash-new',
    'LLM_API_KEY':key,
    'ASR_ENABLED':'true',
    'ASR_URL':'https://aigw.telecomjs.com/api/v1/services/aigc/multimodal-generation/generation',
    'ASR_MODEL':'qwen-audio-3.0-asr-flash',
    'TTS_ENABLED':'true',
    'TTS_URL':'https://aigw.telecomjs.com/api/v3/tts/unidirectional',
    'TTS_MODEL':'seed-tts-2.0',
    'TTS_SPEAKER':'zh_female_vv_uranus_bigtts',
}
for name,value in configuration.items():
    set_key(str(env_path),name,value,quote_mode='always')
print('Configured LLM, ASR and TTS in local .env. Credential was not printed.')
