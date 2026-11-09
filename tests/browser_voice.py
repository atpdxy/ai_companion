"""Test microphone/review/send/play/stop UI using fake microphone and intercepted audio APIs."""
import base64
import io
import json
import os
import uuid
import wave

from playwright.sync_api import expect, sync_playwright

BASE=os.getenv('TEST_WEB_URL','http://localhost:5173')
output=io.BytesIO()
with wave.open(output,'wb') as audio:
    audio.setnchannels(1);audio.setsampwidth(2);audio.setframerate(16000);audio.writeframes(bytes(16000*2*5))

with sync_playwright() as p:
    browser=p.chromium.launch(headless=True,args=['--use-fake-device-for-media-stream','--use-fake-ui-for-media-stream','--autoplay-policy=no-user-gesture-required'])
    context=browser.new_context(permissions=['microphone'],viewport={'width':1200,'height':900})
    page=context.new_page();errors=[];page.on('pageerror',lambda error:errors.append(str(error)))
    page.route('**/api/v1/status',lambda route:route.fulfill(json={'code':'OK','message':'success','request_id':'req_test','data':{'mode':'demo','asr_enabled':True,'tts_enabled':True,'version':'0.2.0'}}))
    captured=[]
    def transcribe(route):
        body=route.request.post_data_json
        assert body['companion_id'] and body['format'] in ('webm','ogg','mp4')
        assert len(base64.b64decode(body['audio_base64']))>100
        captured.append(body['format'])
        route.fulfill(json={'code':'OK','message':'success','request_id':'req_test','data':{'text':'这是录音转写测试','duration_seconds':1}})
    page.route('**/api/v1/audio/transcriptions',transcribe)
    page.route('**/api/v1/conversations/*/messages/*/audio',lambda route:route.fulfill(content_type='audio/wav',body=output.getvalue()))
    registered=False
    try:
        page.goto(BASE);page.wait_for_load_state('networkidle')
        page.get_by_label('邮箱',exact=True).fill(f'voice-{uuid.uuid4().hex}@example.invalid')
        page.get_by_label('密码',exact=True).fill('voice-ui-test-password')
        page.get_by_role('checkbox').check();page.get_by_role('button',name='开启我们的小小世界').click()
        expect(page.get_by_role('navigation')).to_be_visible(timeout=15000);registered=True
        page.get_by_role('button',name='录音转文字',exact=True).click()
        expect(page.get_by_role('button',name='结束录音并转文字')).to_be_visible()
        # Require at least one encoded media chunk before stopping the fake capture.
        page.wait_for_timeout(1100)
        page.get_by_role('button',name='结束录音并转文字').click()
        expect(page.get_by_role('textbox',name='输入你想说的话')).to_have_value('这是录音转写测试',timeout=15000)
        expect(page.locator('.messages article')).to_have_count(0)
        page.get_by_role('button',name='发送消息',exact=True).click()
        expect(page.get_by_role('button',name='朗读这条回复')).to_be_visible(timeout=15000)
        page.get_by_role('button',name='朗读这条回复').click()
        expect(page.get_by_role('button',name='停止朗读')).to_be_visible()
        page.get_by_role('button',name='停止朗读').click()
        expect(page.get_by_role('button',name='朗读这条回复')).to_be_visible()
        assert len(captured)==1 and not errors,(captured,errors)
        print(json.dumps({'status':'passed','fake_microphone':True,'transcriptions':len(captured),'browser_errors':errors}))
    finally:
        if registered:
            response=context.request.delete(BASE+'/api/v1/me',headers={'Origin':BASE},data=json.dumps({'confirmation':'DELETE'}))
            assert response.status==200,response.text()
        context.close();browser.close()
