"""Small live connectivity check using synthetic text only; never prints credentials."""
import asyncio
import base64
import json
import sys
from pathlib import Path

import httpx

from emotional_agent.infrastructure.config import Settings
from emotional_agent.infrastructure.llm import CompatibleGateway
from emotional_agent.media.gateway import TelecomAudioGateway
from emotional_agent.media.schemas import SpeechRequest, TranscriptionRequest


async def main() -> bool:
    root=Path(__file__).resolve().parents[1]
    settings=Settings(_env_file=root/'.env')
    results={}
    async with httpx.AsyncClient(base_url=settings.llm_base_url.rstrip('/')+'/',headers={'Authorization':f'Bearer {settings.llm_api_key}'},timeout=settings.llm_timeout_seconds,follow_redirects=False) as client:
        try:
            gateway=CompatibleGateway(settings,client)
            reply=''.join([part async for part in gateway.stream('接口连接测试，只回复简短中文。','只回复：连接正常')])
            results['llm']={'ok':bool(reply.strip()),'characters':len(reply)}
        except Exception as error:
            results['llm']={'ok':False,'error_type':type(error).__name__}
    async with httpx.AsyncClient(timeout=settings.audio_timeout_seconds,follow_redirects=False) as client:
        gateway=TelecomAudioGateway(settings,client)
        audio=None
        try:
            audio=await gateway.synthesize(SpeechRequest(text='你好，这是一条连接测试。',request_id='req_connectivity'))
            output=root/'.local'/'tts-connectivity.mp3';output.parent.mkdir(exist_ok=True);output.write_bytes(audio)
            results['tts']={'ok':bool(audio),'bytes':len(audio)}
        except Exception as error:
            results['tts']={'ok':False,'error_type':type(error).__name__}
        if audio:
            try:
                result=await gateway.transcribe(TranscriptionRequest(audio_base64=base64.b64encode(audio).decode(),format='mp3',request_id='req_connectivity'))
                results['asr']={'ok':bool(result.text.strip()),'characters':len(result.text),'duration_seconds':result.duration_seconds}
            except Exception as error:
                results['asr']={'ok':False,'error_type':type(error).__name__}
        else:
            results['asr']={'ok':False,'error_type':'no_test_audio'}
    print(json.dumps(results))
    return all(result['ok'] for result in results.values())


if __name__=='__main__':
    sys.exit(0 if asyncio.run(main()) else 1)
