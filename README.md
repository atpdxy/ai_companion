# 知夏 · 智能情感陪伴

依据根目录《智能情感机器人_开发设计文档_V1.0.docx》和《智能情感机器人项目代码开发规范 V1.0.md》实现。产品定位按你的选择调整为**虚拟恋爱陪伴**；默认角色“知夏”，可配置称呼与亲密表达风格。

已实现文字版核心流程和站内提醒。Python 使用 `venv`；Go、AI 与前端独立运行。没有模型密钥也可用演示模式验证完整业务流程，演示回复是固定模板，情绪显示 unknown，不代表真实模型效果。

**V0.2：多伙伴与语音。** 可以创建许多独立 AI 伙伴，每个伙伴使用自己的角色卡、会话、记忆和模拟状态。提供爱人/亲人/朋友模板、角色卡 JSON 导入导出、录音转写和回复朗读；MCP 扩展入口默认关闭，配置后只显示明确授权的只读工具。详见 [多伙伴与模型扩展](docs/companions-and-integrations.md)。

## 快速启动（Windows / PowerShell 7）

需要 Go 1.26+、Node.js 24、Python 3.12+ 和已启动的 Docker Desktop。

```powershell
# 第一次运行；Python 可换成你自己的真实解释器路径
./scripts/setup.ps1 -Python 'C:/Users/atpdx/.local/bin/python3.12.exe'

# 启动 PostgreSQL、Redis、Go、Python 和 Web
./scripts/dev.ps1
```

访问 **http://localhost:5173**，自行注册账号。无预置密码和测试账号。Ctrl+C 停止当前开发进程；数据库容器与数据卷继续保留。

当前电脑 `python` 命令指向 Windows 商店占位程序，已找到 Python 3.12.14 并创建 `services/ai/.venv`。以后运行项目 Python 都使用这个 venv，不依赖全局 `python` 命令。

使用独立 Compose 项目 `zhixia-companion`，避开原有容器：

| 服务 | 本地地址 |
|---|---|
| Web | http://localhost:5173 |
| Go API | http://127.0.0.1:18080 |
| AI 服务 | http://127.0.0.1:18001 |
| PostgreSQL / pgvector | 127.0.0.1:25432 |
| Redis | 127.0.0.1:26379 |

## 接入你的模型

`setup.ps1` 会生成根目录 `.env`，自动随机生成数据库密码、JWT 密钥和内部服务令牌。它不会覆盖已有 `.env`。

编辑 `.env`：

```dotenv
LLM_MODE=live
LLM_BASE_URL=https://aigw.telecomjs.com/v1
LLM_MODEL=deepseek-v4.1-flash-new
LLM_API_KEY=只在本机填写真实密钥
```

`LLM_BASE_URL` 填接口根地址，程序会追加 `/chat/completions`。供应商需要支持聊天 SSE 和 `response_format: {"type":"json_object"}`。不支持结构化输出时，情绪与候选记忆会降级，普通生成仍可工作。修改后重启 AI 与 Go 服务；前端会显示“模型已连接”或“离线演示”。

ASR 用 `ASR_ENABLED=true` 开启，模型为 `qwen-audio-3.0-asr-flash`；TTS 用 `TTS_ENABLED=true` 开启，模型为 `seed-tts-2.0`，默认音色 `zh_female_vv_uranus_bigtts`。两个语音模型使用不同路径和鉴权头，已在适配器中处理。可以运行 `./services/ai/.venv/Scripts/python.exe scripts/configure_gateway.py` 通过隐藏输入配置网关。

`.env` 中可配置超时、重试次数、温度和输出 token 上限。聊天重试不会自动重放已输出的模型流。真实模型的质量、延迟、费用与兼容性需要填写实际服务后验证。

## 已有功能

- 注册、登录、HttpOnly 会话 Cookie、严格同源写请求、Redis 限流与多账户数据隔离。
- 会话列表、多轮上下文、真实 SSE 转发、停止接收、相同 turn_id 重试与已完成轮次恢复。
- 多个独立伙伴与可复用角色卡；按伙伴隔离会话、记忆和模拟状态；每个伙伴可指定不同音色。
- 录音后转写到输入框，确认后发送；手动朗读保存的 AI 回复并随时停止。原始录音不持久化。
- 经 Schema 校验的情绪分析、有限策略、安全/停止门控、稳定人格版本、带时间衰减的模拟状态。
- 记忆默认关闭；开启后提取候选、逐条确认、跨会话召回、纠正、过期和删除；来源可追溯。
- PostgreSQL 与 pgvector 同行保存，内容/向量更新和删除共用事务；没有独立向量服务的删除延迟。
- 数据导出、账号注销、回复反馈、敏感内容不写入日志、跨服务 request_id。
- 用户明确创建的一次性/每日/每周站内提醒，免打扰、每日配额、过期、撤销与幂等。说“别联系我”等停止语会撤销提醒授权。

演示模式下，可先开启记忆，再发送 `请记住：我喜欢下雨天`，到记忆页确认后新建对话询问。没有确认的候选不会参与召回。“不保存”会删除候选。

## Docker 全栈运行

与本地开发模式二选一，避免占用同一组端口：

```powershell
docker compose --env-file .env -f deployments/docker-compose.yml up --build -d
docker compose --env-file .env -f deployments/docker-compose.yml ps
```

访问同样的 http://localhost:5173。Docker 模式只公开 Web、Go API 与绑定回环地址的数据库/Redis，AI 通过容器内网访问。API 启动时在迁移锁保护下顺序执行版本化 SQL。

## 检查与测试

```powershell
./scripts/check.ps1

# 需要开发服务已经运行；测试会创建并清除自己的合成账号
./services/ai/.venv/Scripts/python.exe -m playwright install chromium
./scripts/dev.ps1 -Demo # 先以演示模式启动服务，另一个终端执行下面命令
./scripts/check.ps1 -Integration
```

检查包括 Go 编译/测试/vet、Python Ruff/mypy/pytest、OpenAPI 校验与路由覆盖、TypeScript/ESLint/Prettier/生产构建。安装 `golangci-lint` 后脚本也执行它；CI 配有独立步骤。集成测试覆盖 PostgreSQL/pgvector、Go→Python 通信、越权、记忆生命周期和提醒授权。浏览器截图输出到被 Git 忽略的 `.local/screenshots/`。

更多说明见 [实现说明](docs/implementation.md)、[验收记录](docs/validation.md) 和 [API 契约](packages/contracts/README.md)。

## 当前边界

本版是可运行的研发版本。向量召回使用 128 维字符二元组特征配合词面重排，**还未接入神经网络语义 embedding**；同义表达的召回能力有限。模型候选提取在回复后以独立超时执行，失败时不保存候选；尚未部署独立异步记忆任务队列。

提醒为站内消息，打开“偏好与隐私”查看，每 30 秒刷新；没有邮件、短信或操作系统通知。语音目前为录音转写与短回复朗读，尚无全双工实时通话。Live2D、知识库数据接入、MCP stdio/OAuth、LLM 自主工具规划属于后续扩展。文档中的 300 条情绪评测、人工策略适当率和并发 P95 指标尚未测得，不以连通性测试替代这些指标。
