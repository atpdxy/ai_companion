# 多伙伴、角色卡与模型扩展（V0.2）

本次按用户澄清实现多个独立 AI 伙伴。角色卡是模板，伙伴是独立实例；使用同一张卡创建两个伙伴，也不会共享聊天、记忆或模拟关系状态。

## 数据和迁移

- `character_cards` 保存严格版本化的 `companion.character.v1` JSON 模板，包括名称、关系类型、背景、开场白、表达风格和音色 ID。
- `companions` 保存每个伙伴独立的设定副本与版本，引用创建它的模板。编辑伙伴不修改模板；可显式另存为新模板。
- `conversations.companion_id` 固定会话所属伙伴。选择其他伙伴只改变当前空间，不重新解释已有会话。
- `memories.companion_id` 与 `(user_id, companion_id, content)` 唯一约束保证召回与去重不跨伙伴。模拟状态主键也包含 companion_id。
- 记忆总开关属于账号，但内容按伙伴隔离。界面的“忘记全部记忆”只清除当前伙伴；账号导出/注销仍覆盖全部伙伴。
- 清空记忆更新伙伴自己的时间边界，阻止早先开始的在途轮次再次写入。账号关闭记忆授权仍是全局撤销。
- SQL 005/006 将原有角色迁移为默认伙伴，保留聊天与记忆来源，提供爱人/亲人/朋友三张初始模板。原 `/persona` 路由保留为当前伙伴的兼容接口。

UI 的“AI 伙伴”可增加伙伴、进入各自空间、编辑角色卡设定、导出或导入本项目 JSON 角色卡。此版本未承诺兼容第三方 PNG/SillyTavern 卡格式。

## 三个模型分别适配

| 能力 | 模型 | 路径 | 鉴权/返回 |
|---|---|---|---|
| 对话 | deepseek-v4.1-flash-new | `/v1/chat/completions` | Bearer；OpenAI 兼容 SSE |
| ASR | qwen-audio-3.0-asr-flash | `/api/v1/services/aigc/multimodal-generation/generation` | Bearer；`output.text` |
| TTS | seed-tts-2.0 | `/api/v3/tts/unidirectional` | X-Api-Key + X-Api-Resource-Id + UUID；NDJSON Base64 音频分片 |

TTS 使用后续提供的正式接口路径，不使用最初误填的多模态路径。后端严格解析 code=0 分片并等待 20000000 成功收尾；不完整/业务失败不会冒充完整音频。短回复的 MP3 完整校验后才交给浏览器播放。

ASR 使用 `input_audio.data` 的 Data URI 与必填 format；应用限制单次 10MB、浏览器录音 60 秒，比供应商 5 分钟/2GB 上限更保守。网关继续校验真实格式和时长。原始音频不写数据库/日志/长期文件，内存内完成转写；供应商侧的数据政策独立适用。输入音频只接受 Base64，不接受用户提供的服务器抓取 URL。

交互为：用户点击录音 → 浏览器请求麦克风权限 → 转写填入输入框 → 用户确认发送。语音不会自动发消息。每条已保存的 AI 回复可以手动朗读/停止，使用该消息所属伙伴的 voice_id；留空采用服务端 `TTS_SPEAKER`。当前同步朗读限制 1000 字，不自动截断；不支持长文任务或全双工实时通话。

三项服务可使用不同密钥；ASR_API_KEY/TTS_API_KEY 留空时复用 LLM_API_KEY。密钥只在根目录被 Git 忽略的 `.env`。`scripts/configure_gateway.py` 隐藏输入，不把密钥放进命令行参数。

```powershell
# 只使用合成文本的小型真实连通性检查，会调用三项付费服务
./services/ai/.venv/Scripts/python.exe scripts/check_gateway.py

# 回归测试用演示进程，与 .env 中的真实配置分开
./scripts/dev.ps1 -Demo
```

## MCP 的边界

MCP 用于扩展工具与外部数据，不承担大模型/ASR/TTS 的统一推理协议。当前接入官方 Python MCP SDK 1.x 的 Streamable HTTP 传输，保留独立 ModelGateway、SpeechRecognition、SpeechSynthesis 接口。

默认 `MCP_SERVERS_FILE` 为空，不连接任何外部 MCP 服务。管理员复制 `deployments/mcp.servers.example.json` 到 `.local/mcp.servers.json`，逐服务器配置：

1. 明确的 HTTPS 或本机回环 HTTP URL。
2. allowed_user_ids 逐账号授权，空列表不授权任何用户，不支持通配符。
3. tools 白名单和管理员声明的 read_only=true；写工具拒绝执行。
4. token_env 指向单独的环境变量，不复用用户 Cookie、JWT 或模型密钥。

设置 `MCP_SERVERS_FILE=.local/mcp.servers.json` 后重启 AI 服务。工具调用有独立超时、JSON Schema 参数校验和输入/输出大小限制。设置页“扩展工具与数据源”只显示当前账号获准工具，点击调用才执行，结果显示为不可信数据。没有让模型自主调用外部写操作。

Docker 配置需要单独绑定配置文件并传入其 token_env 对应变量，例如：

```yaml
services:
  ai:
    environment:
      MCP_SERVERS_FILE: /app/config/mcp.servers.json
      KNOWLEDGE_MCP_TOKEN: ${KNOWLEDGE_MCP_TOKEN}
    volumes:
      - ../.local/mcp.servers.json:/app/config/mcp.servers.json:ro
```

上述配置应放入本机 override 文件；先创建源配置文件再挂载，不提交真实令牌。

## 验证范围

已用用户指定网关完成合成文本测试：大模型流式返回成功；TTS 生成 18,093 字节 MP3；ASR 转写这段 2 秒测试音频成功。没有把真实用户聊天用于连通性验证。这是接口连通性证据，不替代质量、费用、负载和长期稳定性评测。

另外通过 `scripts/check_live_api.py` 验证了真实业务 API 的完整链路与语音拥有者校验。此脚本会创建并清除自己的合成账号，会进行少量真实模型调用。

离线测试覆盖跨伙伴记忆/状态归属、模板权限、ASR 请求、TTS 鉴权/结束事件/中断、MCP 用户白名单/写工具拒绝/Schema 校验。浏览器以假麦克风与拦截的语音接口验证录音、确认发送、播放和停止；另用真实网关验证供应商适配器。

尚未提供实际 MCP 服务器，所以外部 MCP 实连待配置后验证。未实现 MCP stdio、OAuth 自动授权、LLM 自主工具规划、第三方卡格式或全双工语音。

参考：[MCP 架构](https://modelcontextprotocol.io/docs/learn/architecture)、[官方 Python SDK](https://github.com/modelcontextprotocol/python-sdk)。语音协议以用户粘贴的 [ASR](https://token.telecomjs.com/modelDetail?id=367) 与 [TTS](https://token.telecomjs.com/modelDetail?id=358) 文档为依据。
