"""Live Go→AI→gateway smoke with synthetic accounts; small paid model calls, no real chat data."""
import base64
import json
import uuid

import httpx

BASE='http://127.0.0.1:18080'
ORIGIN='http://localhost:5173'


def data(response: httpx.Response):
    response.raise_for_status()
    return response.json()['data']


def main():
    clients=[]
    results={}
    try:
        for _ in range(2):
            client=httpx.Client(base_url=BASE,headers={'Origin':ORIGIN},timeout=120)
            data(client.post('/api/v1/auth/register',json={'email':f'live-smoke-{uuid.uuid4().hex}@example.invalid','password':'synthetic-live-smoke-password','adult_confirmed':True}))
            clients.append(client)
        owner,other=clients
        status=data(owner.get('/api/v1/status'))
        assert status['mode']=='live' and status['tts_enabled'] and status['asr_enabled'],'Start live mode before this check.'
        cards=data(owner.get('/api/v1/character-cards'))
        card=next(card for card in cards if card['definition']['relationship']=='friend')
        companion=data(owner.post('/api/v1/companions',json={'name':'连通性朋友','card_id':card['id']}))
        conversation=data(owner.post('/api/v1/conversations',json={'title':'合成连通性测试','companion_id':companion['id']}))
        reply=data(owner.post(f"/api/v1/conversations/{conversation['id']}/messages",json={'content':'你好，这是接口连通性测试，请用一句简短的朋友式问候回复。','turn_id':uuid.uuid4().hex}))
        assert reply['mode']=='live' and reply['response'].strip()
        results['api_llm']={'ok':True,'characters':len(reply['response'])}
        messages=data(owner.get(f"/api/v1/conversations/{conversation['id']}/messages"))
        assistant=next(message for message in messages if message['role']=='assistant')
        path=f"/api/v1/conversations/{conversation['id']}/messages/{assistant['id']}/audio"
        assert other.post(path).status_code==404
        speech=owner.post(path);speech.raise_for_status()
        assert speech.headers['content-type'].startswith('audio/mpeg') and speech.content
        results['api_tts']={'ok':True,'bytes':len(speech.content)}
        body={'companion_id':companion['id'],'format':'mp3','audio_base64':base64.b64encode(speech.content).decode()}
        assert other.post('/api/v1/audio/transcriptions',json=body).status_code==404
        transcription=data(owner.post('/api/v1/audio/transcriptions',json=body))
        assert transcription['text'].strip()
        results['api_asr']={'ok':True,'characters':len(transcription['text'])}
        results['cross_user_audio_denied']=True
        print(json.dumps(results))
    finally:
        for client in clients:
            response=client.request('DELETE','/api/v1/me',json={'confirmation':'DELETE'})
            response.raise_for_status()
            client.close()


if __name__=='__main__':
    main()
