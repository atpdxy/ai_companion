"""Browser regression for independent companions, scoped memories and reusable cards."""
import json
import os
import uuid
from pathlib import Path

from playwright.sync_api import expect, sync_playwright

BASE=os.getenv('TEST_WEB_URL','http://localhost:5173')
ROOT=Path(__file__).resolve().parents[1]
OUTPUT=ROOT/'.local/screenshots'
OUTPUT.mkdir(parents=True,exist_ok=True)

with sync_playwright() as p:
    browser=p.chromium.launch(headless=True)
    context=browser.new_context(viewport={'width':1440,'height':1000},locale='zh-CN')
    page=context.new_page();errors=[];page.on('pageerror',lambda error:errors.append(str(error)))
    page.goto(BASE);page.wait_for_load_state('networkidle')
    registered=False
    try:
        page.get_by_label('邮箱',exact=True).fill(f'companions-{uuid.uuid4().hex}@example.invalid')
        page.get_by_label('密码',exact=True).fill('companion-browser-password')
        page.get_by_role('checkbox').check();page.get_by_role('button',name='开启我们的小小世界').click()
        expect(page.get_by_role('navigation')).to_be_visible(timeout=15000);registered=True
        page.get_by_role('navigation').get_by_role('button',name='我们的记忆',exact=True).click()
        page.get_by_role('switch',name='允许记忆',exact=True).click()
        expect(page.get_by_role('switch',name='允许记忆',exact=True)).to_have_attribute('aria-checked','true')
        page.get_by_role('button',name='此刻的陪伴',exact=True).click()
        page.get_by_role('textbox',name='输入你想说的话').fill('请记住：我喜欢海边散步')
        page.get_by_role('button',name='发送消息',exact=True).click()
        expect(page.get_by_role('textbox',name='输入你想说的话')).to_be_enabled(timeout=15000)
        page.get_by_role('navigation').get_by_role('button',name='我们的记忆').click()
        page.get_by_role('button',name='允许记住',exact=True).click();expect(page.get_by_text('已获准记住',exact=True)).to_be_visible()
        page.get_by_role('button',name='AI 伙伴',exact=True).click()
        page.get_by_role('button',name='添加伙伴',exact=True).click()
        page.get_by_label('伙伴名字',exact=True).fill('阿舟')
        selector=page.get_by_label('使用的角色卡模板',exact=True)
        option=selector.locator('option').filter(has_text='虚拟朋友').first
        selector.select_option(option.get_attribute('value'))
        page.get_by_role('button',name='创建独立伙伴',exact=True).click()
        expect(page.get_by_role('heading',name='阿舟',exact=True)).to_be_visible(timeout=15000)
        expect(page.locator('.history button')).to_have_count(0)
        page.get_by_role('navigation').get_by_role('button',name='我们的记忆',exact=True).click()
        expect(page.get_by_role('heading',name='还没有留下记忆')).to_be_visible()
        page.get_by_role('button',name='AI 伙伴',exact=True).click()
        expect(page.locator('.companion-card')).to_have_count(2)
        page.screenshot(path=str(OUTPUT/'companions-desktop.png'),full_page=True,animations='disabled')
        page.get_by_role('button',name='将当前设定存为角色卡').click()
        expect(page.get_by_role('alert')).to_have_count(0)
        lover=page.locator('.companion-card').filter(has=page.get_by_role('heading',name='知夏',exact=True))
        lover.get_by_role('button',name='和 TA 聊聊').click()
        expect(page.get_by_role('heading',name='知夏',exact=True)).to_be_visible()
        page.get_by_role('navigation').get_by_role('button',name='我们的记忆',exact=True).click()
        expect(page.get_by_role('heading',name='我喜欢海边散步',exact=True)).to_be_visible()
        page.get_by_role('button',name='AI 伙伴',exact=True).click()
        # A delayed save for the old companion must not overwrite the newly selected companion.
        original=context.request.get(BASE+'/api/v1/persona').json()['data']
        held=[]
        def hold_save(route):
            if route.request.method=='PUT': held.append(route)
            else: route.continue_()
        page.route(f"**/api/v1/companions/{original['id']}",hold_save)
        page.get_by_label('TA 的名字',exact=True).fill('慢请求的旧伙伴')
        page.get_by_role('button',name='保存这份默契').click()
        for _ in range(20):
            if held: break
            page.wait_for_timeout(50)
        assert held
        friend=page.locator('.companion-card').filter(has=page.get_by_role('heading',name='阿舟',exact=True))
        friend.get_by_role('button',name='和 TA 聊聊').click()
        expect(page.get_by_role('heading',name='阿舟',exact=True)).to_be_visible()
        held[0].fulfill(json={'code':'OK','message':'success','request_id':'req_delayed','data':{**original,'name':'慢请求的旧伙伴'}})
        page.wait_for_timeout(150)
        expect(page.get_by_role('heading',name='阿舟',exact=True)).to_be_visible()
        expect(page.get_by_role('heading',name='慢请求的旧伙伴',exact=True)).to_have_count(0)
        page.get_by_role('button',name='AI 伙伴',exact=True).click()
        page.set_viewport_size({'width':390,'height':844})
        page.screenshot(path=str(OUTPUT/'companions-mobile.png'),full_page=True,animations='disabled')
        assert page.evaluate('document.documentElement.scrollWidth<=window.innerWidth')
        assert not errors,errors
        print(json.dumps({'status':'passed','independent_companions':2,'browser_errors':errors}))
    finally:
        if registered:
            response=context.request.delete(BASE+'/api/v1/me',headers={'Origin':BASE},data=json.dumps({'confirmation':'DELETE'}))
            assert response.status==200,response.text()
        context.close();browser.close()
