# 验收记录

V0.2 增量检查：Python 30 项测试、Go 12 项测试、HTTP 3 个场景和 3 组浏览器流程通过。新增检查包括伙伴/角色卡拥有者隔离、跨伙伴记忆独立、切换时旧请求的迟到响应、ASR 协议、TTS NDJSON/失败收尾、MCP 权限/写工具拒绝/禁止外部 Schema 引用抓取、配置错误不暴露密钥，以及假麦克风的录音确认发送/朗读停止。真实网关的 LLM 流式、TTS、ASR 小样本均成功，详情见 [V0.2 说明](companions-and-integrations.md)。

V0.2 容器模式也通过相同的 HTTP 3 场景及基础/多伙伴/语音三组浏览器流程。恢复本地 Go + Python venv 后，`scripts/check_live_api.py` 进一步验证真实 Go→Python→模型网关链路：生成 24 字测试回复、TTS 返回 37,293 字节 MP3、ASR 返回 23 字转写；跨用户朗读和转写请求均返回 404。所有合成测试账号均已清除，当前本地服务运行于 live 模式。

2026-11-09 在本机离线演示模式完成检查，不连接收费模型；无真实用户数据。

结果：Python 19 项测试通过；Go 11 项测试（含 PostgreSQL 集成与上游完整/中断两个子场景）通过；HTTP 两项端到端场景通过；Chromium 全流程通过且零脚本异常。Ruff、mypy、gofmt、go vet、golangci-lint、ESLint、Prettier、TypeScript、Web 生产构建和两份 OpenAPI 校验均通过。三个 Docker 镜像成功构建，Compose 全栈启动成功；容器模式再次通过相同 HTTP 和浏览器端到端测试。

| 层 | 检查 |
|---|---|
| Python | Ruff、mypy、pytest；无效模型输出、有限重试、流中断、策略、敏感记忆过滤、状态范围与内部鉴权 |
| Go | gofmt、go vet、单元测试；JWT、Origin、输入校验、SSE、Go→Python 客户端 |
| 数据库 | 实际 PostgreSQL/pgvector；会话幂等、拥有者隔离、记忆确认/纠正/删除、在途撤销、提醒准入与去重 |
| HTTP | 实际 Go + Python + Redis；完整聊天与跨会话记忆、越权、数据导出、注销与停止提醒 |
| 前端 | TypeScript、ESLint、Prettier、Vite 生产构建 |
| 浏览器 | 实际 Chromium；注册、聊天、确认/纠正/删除记忆、人格保存、提醒授权；1440×1000 与 390×844 |
| 契约 | 两份 OpenAPI Schema 校验；检查 Go 注册路由与契约完全覆盖 |

浏览器截图保存在 `.local/screenshots/`。测试账号只使用 `example.invalid` 合成地址，执行后删除。原有其他项目的容器与数据库不参与测试。

尚未执行文档要求的 300 条独立中文标注集质量评测、人工双人策略评测、20 并发真实模型压测及长期可用性观测。演示模式不能作为模型质量或生产 SLO 的证据。
