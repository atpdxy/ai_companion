# API 契约

`openapi.json`：Go 对外 API；`ai.openapi.json`：Python 内部 API。

```powershell
./services/ai/.venv/Scripts/python.exe scripts/generate_contracts.py
```

公共接口统一 Envelope：`code/message/data/request_id`。HTTP 400/401/404/409/429/503 分别表示输入错误、未认证、不存在或非本人、幂等冲突、限流和上游不可用。Cookie 写请求必须附精确的 `Origin: http://localhost:5173`；可在受信 API 客户端使用 Bearer JWT。

注册设置 HttpOnly Cookie。必须明确 `adult_confirmed: true`，密码 10–72 UTF-8 字节。内部 Python 使用独立服务令牌，只接收 Go 验证过的用户范围数据。

发送消息字段：`content` 与客户端生成的 `turn_id`。`Accept: text/event-stream` 返回 `message_start`、`delta`、`message_end`、`error`；不带此 Accept 返回完整 JSON。POST 的 `turn_id` 必须在重试时保持不变。恢复接口为完成轮次快照，详见 docs/implementation.md。

时间为 ISO 8601 带时区格式。关闭记忆总开关会停止提取/召回并清除待确认项，保留已批准记录供用户管理。`PATCH memory` 的 rejected 状态删除候选，返回 data=null。记忆过期字段可设 null；后台检索拒绝过期记录。

V0.2 增加 `/companions`（独立 AI 伙伴）和 `/character-cards`（可复用设定模板）。新会话可以显式指定 companion_id；创建后永久绑定该伙伴，当前选择的伙伴变化不会改动已有会话。GET memories 默认仅返回当前伙伴；DELETE memories 附 companion_id 时只清理对应伙伴，不带时是账号级清除。

语音入口：POST `/audio/transcriptions` 接收 companion_id、audio_base64 和 format（10MB 上限）；POST `/conversations/{id}/messages/{message}/audio` 仅朗读本人、对应会话的 assistant 消息，返回 audio/mpeg，1000 字上限。模型密钥始终保留在后端。

MCP 入口：GET `/integrations/tools?companion_id=...` 发现本人已授权工具；POST `/integrations/tools/invoke` 显式调用只读工具。默认配置为空，发现结果为空列表。用户、伙伴、服务器白名单和 JSON Schema 均需通过校验。当前没有让 LLM 自主执行工具的路径。
