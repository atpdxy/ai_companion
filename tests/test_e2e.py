"""HTTP integration against running local demo services. Creates/deletes only test accounts."""
import os
import time
import uuid
from datetime import UTC, datetime, timedelta

import httpx
import pytest

BASE = os.getenv('TEST_API_URL', 'http://127.0.0.1:18080')
ORIGIN = os.getenv('WEB_ORIGIN', 'http://localhost:5173')


@pytest.fixture(scope='module')
def accounts():
    clients = [httpx.Client(base_url=BASE, headers={'Origin': ORIGIN}, timeout=150) for _ in range(2)]
    for client in clients:
        response = client.post('/api/v1/auth/register', json={'email': f'e2e-{uuid.uuid4().hex}@example.invalid', 'password': 'test-password-not-for-real-use', 'adult_confirmed': True})
        assert response.status_code == 201, response.text
        assert response.cookies.get('companion_session')
    yield clients
    for client in clients:
        response = client.request('DELETE', '/api/v1/me', json={'confirmation':'DELETE'})
        assert response.status_code == 200
        assert client.get('/api/v1/me').status_code == 401
        client.close()


def data(response: httpx.Response):
    assert response.is_success, response.text
    envelope = response.json()
    assert envelope['code'] == 'OK' and envelope['request_id'].startswith('req_')
    return envelope['data']


def test_complete_conversation_memory_and_isolation(accounts):
    owner, other = accounts
    conversation = data(owner.post('/api/v1/conversations', json={'title':'端到端验收'}))['id']
    assert other.get(f'/api/v1/conversations/{conversation}/messages').status_code == 404
    assert data(owner.get('/api/v1/me'))['memory_consent'] is False
    data(owner.patch('/api/v1/me/preferences', json={'memory_consent': True}))
    turn = uuid.uuid4().hex
    body = {'content':'请记住：我喜欢下雨天', 'turn_id':turn}
    reply = data(owner.post(f'/api/v1/conversations/{conversation}/messages', json=body))
    assert reply['mode'] == 'demo'
    assert reply['emotion']['label'] == 'unknown'
    replay = data(owner.post(f'/api/v1/conversations/{conversation}/messages', json=body))
    assert replay['response'] == reply['response'] and 'emotion' not in replay
    messages = data(owner.get(f'/api/v1/conversations/{conversation}/messages'))
    assert len(messages) == 2
    assert owner.post(f'/api/v1/conversations/{conversation}/messages', json={**body, 'content':'different'}).status_code == 409
    memory = data(owner.get('/api/v1/memories'))[0]
    assert memory['consent_status'] == 'pending' and memory['source_message_id'] == messages[0]['id']
    assert data(other.get('/api/v1/memories')) == []
    update = {'content':'我喜欢晴天', 'consent_status':'approved', 'expires_at':None}
    assert other.patch(f"/api/v1/memories/{memory['id']}", json=update).status_code == 404
    data(owner.patch(f"/api/v1/memories/{memory['id']}", json=update))
    second = data(owner.post('/api/v1/conversations', json={'title':'跨会话'}))['id']
    reply = data(owner.post(f'/api/v1/conversations/{second}/messages', json={'content':'记得我喜欢晴天吗', 'turn_id':uuid.uuid4().hex}))
    assert memory['id'] in reply['retrieved_memory_ids'] and '晴天' in reply['response']
    assert other.delete(f"/api/v1/memories/{memory['id']}").status_code == 404
    data(owner.delete(f"/api/v1/memories/{memory['id']}"))
    assert data(owner.get('/api/v1/memories')) == []
    reply = data(owner.post(f'/api/v1/conversations/{second}/messages', json={'content':'晴天', 'turn_id':uuid.uuid4().hex}))
    assert reply['retrieved_memory_ids'] == []
    stream = owner.get(f'/api/v1/conversations/{conversation}/stream', params={'turn_id':turn})
    assert 'event: message_end' in stream.text and 'sadness' not in stream.text
    restored = owner.get(f'/api/v1/conversations/{conversation}/stream', params={'turn_id':turn}, headers={'Last-Event-ID':'2'})
    assert 'event: delta' not in restored.text and 'event: message_end' in restored.text
    assert other.get(f'/api/v1/conversations/{conversation}/stream', params={'turn_id':turn}).status_code == 404
    assert owner.post('/api/v1/conversations', json={}, headers={'Origin':'https://evil.invalid'}).status_code == 401
    assert owner.post('/api/v1/feedback', json={'message_id':messages[1]['id'], 'rating':1}).status_code == 201
    assert other.post('/api/v1/feedback', json={'message_id':messages[1]['id'], 'rating':1}).status_code == 404
    exported = data(owner.get('/api/v1/me/export'))
    assert 'password' not in str(exported['user'])


def test_sse_stop_and_notification_revocation(accounts):
    owner, other = accounts
    prefs = {'opt_in':True,'no_contact':False,'quiet_start':0,'quiet_end':0,'daily_limit':3,'timezone':'Asia/Shanghai'}
    data(owner.put('/api/v1/notification-preferences', json=prefs))
    due = datetime.now(UTC)+timedelta(seconds=2)
    reminder = {'content':'记得喝水', 'due_at':due.isoformat(), 'recurrence':'none', 'client_key':uuid.uuid4().hex}
    event = data(owner.post('/api/v1/events', json=reminder))
    assert other.delete(f"/api/v1/events/{event['id']}").status_code == 404
    conversation = data(owner.post('/api/v1/conversations', json={}))['id']
    response = owner.post(f'/api/v1/conversations/{conversation}/messages', json={'content':'别再问了，我想安静一会', 'turn_id':uuid.uuid4().hex}, headers={'Accept':'text/event-stream'})
    assert response.status_code == 200
    assert 'respect_space' in response.text and 'event: message_end' in response.text
    assert data(owner.get('/api/v1/notification-preferences'))['opt_in'] is False
    time.sleep(11)
    assert data(owner.get('/api/v1/notifications')) == []
    assert owner.post('/api/v1/events', json={**reminder,'due_at':(datetime.now(UTC)+timedelta(minutes=1)).isoformat(), 'client_key':uuid.uuid4().hex}).status_code == 400
    data(owner.delete(f"/api/v1/events/{event['id']}"))


def test_independent_companions_and_tool_permissions(accounts):
    owner,other=accounts
    original=data(owner.get('/api/v1/persona'))
    cards=data(owner.get('/api/v1/character-cards'))
    friend_card=next(card for card in cards if card['definition']['relationship']=='friend')
    friend=data(owner.post('/api/v1/companions',json={'card_id':friend_card['id'],'name':'林舟'}))
    assert friend['id']!=original['id']
    assert other.get(f"/api/v1/companions/{friend['id']}").status_code==404
    assert other.post('/api/v1/companions',json={'card_id':friend_card['id'],'name':'stolen'}).status_code==404
    conversation=data(owner.post('/api/v1/conversations',json={'title':'朋友独立空间','companion_id':friend['id']}))
    assert conversation['companion_id']==friend['id']
    data(owner.patch('/api/v1/me/preferences',json={'memory_consent':True}))
    reply=data(owner.post(f"/api/v1/conversations/{conversation['id']}/messages",json={'content':'请记住：我喜欢骑自行车','turn_id':uuid.uuid4().hex}))
    memory=data(owner.get('/api/v1/memories',params={'companion_id':friend['id']}))[0]
    data(owner.patch(f"/api/v1/memories/{memory['id']}",json={'content':memory['content'],'consent_status':'approved','expires_at':None}))
    assert all(item['id']!=memory['id'] for item in data(owner.get('/api/v1/memories',params={'companion_id':original['id']})))
    data(owner.post(f"/api/v1/companions/{original['id']}/activate"))
    # Changing selected companion must not rebind the existing friend conversation.
    reply=data(owner.post(f"/api/v1/conversations/{conversation['id']}/messages",json={'content':'还记得我喜欢骑自行车吗','turn_id':uuid.uuid4().hex}))
    assert memory['id'] in reply['retrieved_memory_ids']
    assert data(owner.get('/api/v1/integrations/tools',params={'companion_id':friend['id']}))==[]
    assert other.get('/api/v1/integrations/tools',params={'companion_id':friend['id']}).status_code==404
    assert owner.post('/api/v1/integrations/tools/invoke',json={'companion_id':friend['id'],'tool_id':'unconfigured:read','arguments':{}}).status_code==400
    data(owner.delete('/api/v1/memories',params={'companion_id':friend['id']}))
    assert data(owner.get('/api/v1/memories',params={'companion_id':friend['id']}))==[]
