"""Real browser workflow; synthetic account is permanently removed on completion."""
import json
import os
import re
import uuid
from pathlib import Path

from playwright.sync_api import expect, sync_playwright

ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / '.local' / 'screenshots'
OUTPUT.mkdir(parents=True, exist_ok=True)
BASE = os.getenv('TEST_WEB_URL', 'http://localhost:5173')
EMAIL = f'browser-{uuid.uuid4().hex}@example.invalid'

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    context = browser.new_context(viewport={'width':1440,'height':1000}, locale='zh-CN', timezone_id='Asia/Shanghai')
    page = context.new_page()
    errors = []
    page.on('pageerror', lambda error: errors.append(str(error)))
    page.goto(BASE)
    page.wait_for_load_state('networkidle')
    page.screenshot(path=str(OUTPUT/'login-desktop.png'), full_page=True, animations='disabled')
    registered = False
    try:
        page.get_by_label('邮箱', exact=True).fill(EMAIL)
        page.get_by_label('密码', exact=True).fill('browser-test-password-123')
        page.get_by_role('checkbox').check()
        page.get_by_role('button', name='开启我们的小小世界').click()
        expect(page.get_by_role('navigation', name='主导航')).to_be_visible(timeout=15000)
        registered = True
        expect(page.get_by_role('heading', name=re.compile(r'今天的你，\s*想从哪里说起？'))).to_be_visible(timeout=15000)
        page.screenshot(path=str(OUTPUT/'welcome-desktop.png'), full_page=True, animations='disabled')
        assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth')
        page.get_by_role('navigation').get_by_role('button', name='我们的记忆', exact=True).click()
        page.get_by_role('switch', name='允许记忆', exact=True).click()
        expect(page.get_by_role('switch', name='允许记忆', exact=True)).to_have_attribute('aria-checked','true')
        page.get_by_role('button', name='此刻的陪伴').click()
        page.get_by_role('textbox', name='输入你想说的话').fill('请记住：我喜欢下雨天')
        page.get_by_role('button', name='发送消息', exact=True).click()
        expect(page.get_by_text('离线演示 · 非真实模型')).to_be_visible(timeout=15000)
        page.get_by_role('navigation').get_by_role('button', name=re.compile('我们的记忆')).click()
        expect(page.get_by_text('等待你确认', exact=True)).to_be_visible()
        page.get_by_role('button', name='允许记住', exact=True).click()
        expect(page.get_by_text('已获准记住', exact=True)).to_be_visible()
        page.get_by_role('button', name='纠正', exact=True).click()
        page.get_by_role('textbox', name='纠正记忆内容').fill('我喜欢晴天')
        page.get_by_role('button', name='保存纠正').click()
        expect(page.get_by_role('heading', name='我喜欢晴天', exact=True)).to_be_visible()
        page.screenshot(path=str(OUTPUT/'memories-desktop.png'), full_page=True, animations='disabled')
        page.get_by_role('button', name='开启一段对话').click()
        page.get_by_role('textbox', name='输入你想说的话').fill('你记得我喜欢晴天吗')
        page.get_by_role('button', name='发送消息', exact=True).click()
        expect(page.get_by_text('你，我记得你之前提过：我喜欢晴天。愿意的话，我们可以从这里继续聊。',exact=True)).to_be_visible(timeout=15000)
        expect(page.get_by_role('textbox',name='输入你想说的话')).to_be_enabled(timeout=15000)
        page.screenshot(path=str(OUTPUT/'conversation-desktop.png'), full_page=True, animations='disabled')
        page.get_by_role('button', name='AI 伙伴').click()
        page.get_by_label('TA 的名字', exact=True).fill('晚晴')
        page.get_by_label('希望 TA 怎样称呼你', exact=True).fill('小雨')
        page.get_by_role('button',name='保存这份默契').click()
        expect(page.get_by_role('status')).to_have_text('TA 会以新的风格陪伴你')
        page.get_by_role('button', name='偏好与隐私').click()
        expect(page.get_by_role('switch', name='允许站内提醒')).to_have_attribute('aria-checked','false')
        page.get_by_role('switch', name='允许站内提醒').click()
        expect(page.get_by_role('heading',name='约定一个提醒')).to_be_visible()
        page.screenshot(path=str(OUTPUT/'settings-desktop.png'), full_page=True, animations='disabled')
        page.get_by_role('navigation').get_by_role('button', name='我们的记忆', exact=True).click()
        page.get_by_role('button',name='删除这条记忆').click()
        expect(page.get_by_role('heading',name='还没有留下记忆')).to_be_visible()
        page.get_by_role('button', name='此刻的陪伴').click()
        page.set_viewport_size({'width':390,'height':844})
        page.screenshot(path=str(OUTPUT/'conversation-mobile.png'), full_page=True, animations='disabled')
        assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth')
        page.get_by_role('button', name='打开导航').click()
        page.get_by_role('button', name='开启一段对话').click()
        expect(page.get_by_role('heading', name=re.compile(r'今天的你，\s*想从哪里说起？'))).to_be_visible()
        page.screenshot(path=str(OUTPUT/'welcome-mobile.png'), full_page=True, animations='disabled')
        page.get_by_role('textbox',name='输入你想说的话').fill('unsent private draft')
        page.get_by_role('button',name='打开导航').click()
        page.get_by_role('button',name='退出登录').click()
        page.get_by_role('button',name='已经来过？登录').click()
        page.get_by_label('邮箱',exact=True).fill(EMAIL)
        page.get_by_label('密码',exact=True).fill('browser-test-password-123')
        page.get_by_role('button',name='回到我们的空间',exact=True).click()
        expect(page.get_by_role('textbox',name='输入你想说的话')).to_have_value('',timeout=15000)
        assert not errors, errors
        print(json.dumps({'status':'passed','screenshots':str(OUTPUT),'browser_errors':errors},ensure_ascii=False))
    finally:
        if registered:
            response = context.request.delete(BASE+'/api/v1/me', headers={'Origin':BASE}, data=json.dumps({'confirmation':'DELETE'}))
            assert response.status == 200, response.text()
        context.close()
        browser.close()
