# 实现说明与设计映射

> V0.2 已扩展为多个独立伙伴、可复用角色卡和语音，并加入默认关闭的 MCP 扩展入口。详细迁移、配置与当前边界见 [多伙伴与模型扩展](companions-and-integrations.md)。下方的 V0.1 历史差异以该说明为准。

## 输入和决策

原始 Word 及 Markdown 保持不变。Word 的产品/架构要求作为需求依据，Markdown 作为代码规范。用户进一步指定 Python 使用 venv、使用已安装的 Go/Node，以及产品偏向虚拟恋爱陪伴；这些决定优先于文档中的“待确定”选项。

目录采用代码规范中的 `services/api`、`services/ai/src/emotional_agent` 和 `apps/web`。Go 使用标准库 HTTP 路由，避免额外 Web 框架；React/Vite 是设计允许的前端选项；FastAPI/Pydantic 为设计指定技术。

## 分层

| 层 | 实现 |
|---|---|
| 协议入口 | Go internal/handler；Python api |
| 应用流程 | Go internal/service；Python agents/orchestrator |
| 领域规则 | Go internal/domain；Python emotion/memory/personality/state/safety |
| 依赖接口 | Go internal/repository；Python ModelGateway Protocol |
| 基础设施 | pgx PostgreSQL、Redis、AI HTTP Client、兼容 LLM Gateway |

Go 负责身份、拥有者约束、数据持久化、调度与转发。Python 负责情绪、候选记忆、召回重排、策略与语言生成；不接受用户 JWT，不直接访问数据库。内部鉴权使用与 JWT 密钥不同的服务令牌。

## 一轮聊天

1. 验证身份、Origin、限流、会话归属及 turn_id。
2. 短事务锁定会话，写入用户消息并占用 processing 轮次；每会话只允许一个未完成轮次。
3. 取最近 20 条上下文、人格和模拟状态；仅授权开启时查询本人已批准、可信且未过期的记忆。
4. Python 并发执行情绪识别与重排。未知或超时输出 unknown，不作情绪事实判断。
5. 停止优先，其次风险门控、用户需求、情绪和默认倾听。策略只允许有限枚举。
6. 生成器真实消费供应商 SSE，短尾部缓冲检测分块的越界短语。客户端断开会沿 context 取消调用。
7. 生成后进行有界候选提取；来源引文必须出现在当前用户消息，置信度低于 0.8 或敏感内容丢弃。
8. Go 在事务内保存回复、模拟状态和待确认候选；重新检查记忆授权及清除时间，再提交。提交成功后才发送 message_end。

源模型输出和工具结果均作为不可信数据；没有可执行的 LLM 指令通道。人格名字/称呼等可编辑字段仅作为序列化数据进入提示，不能覆盖核心规则。停止与风险回应直接由规则生成，跳过普通生成。

## 幂等与断线

turn_id 是全局唯一键，记录拥有者、会话和原文摘要。重复键配不同文本返回 409。已完成轮次返回原回复；incomplete 轮次重用用户消息，不重复入库。3 分钟的 processing 租约允许进程崩溃后重试；模型与请求预算小于租约。

`POST messages` 通过 Accept 选择 JSON/SSE。`GET stream?turn_id=...` 恢复**已完成轮次的完整快照**，不是逐 token 的事件日志。逻辑事件 ID 为 start=1、delta=2、end=3；多个实时 delta 共用 ID 2，message_end 携带全文，恢复客户端必须用全文替换局部文本。处理中返回 409，未完成轮次使用原 turn_id 重发 POST。

## 记忆与删除

- 用户总开关控制提取及召回；每条候选另需确认。拒绝候选直接删除；纠正会更新置信度为用户确认的 1。
- source_message_id 和 source_quote 保留来源。删除记忆不会删除聊天原文；当前会话短期上下文可能仍包含用户先前说过的话。全量删除账号才会级联删除聊天、记忆、索引、提醒与审计。
- pgvector 列直接位于 memories 主记录中，无外部缓存或独立向量库。更新/删除事务同时处理权威记录与向量，因而无需分布式 outbox。后续拆到 Qdrant 时必须引入删除 outbox。
- 关闭授权和清空全部记忆会更新时间边界；早于清除开始的在途轮次不能重新写入候选。生成中的模型已接收的上下文无法追溯撤回，后续请求使用新的授权快照。
- 检索先按 user_id、consent、confidence、expiry 过滤，pgvector 字符特征排序最多 50 条，Python 再重排取 5 条。此版本不是神经语义检索；不承诺同义表达覆盖。
- 事件记忆保存原话，不将“周五”等相对日期猜成确定日期。提醒必须在独立表单明确指定有时区时间。

## 隐私与状态

情绪推断不持久化到历史轮次/消息/日志。当前响应可展示不确定性，重放时不再返回推断。模拟状态是有界数值，不等同真实情感；按固定规则和时间衰减变化，不包含“用户不回复就生气”的规则。

日志只包含事件、服务、trace、耗时和错误码，不包含聊天全文、记忆、密码和上游错误体。HttpOnly/SameSite Strict Cookie 配合精确 Origin 检查；生产使用 HTTPS Origin 时自动启用 Secure Cookie。部署边界的 TLS 由反向代理提供。

## P1 站内提醒

独立于记忆授权。只允许用户创建的明确内容与时间；支持一次、每日、每周和有效期。Go 可取消调度循环每 10 秒运行；执行事务锁定事件与偏好，再检查 opt-in/no-contact、用户时区、跨日免打扰、每日配额、撤销和有效期。唯一(event_id,occurrence_at)避免重复；原始到期时间另存以支持周期任务的幂等创建。

被延后的任务用 next_check_at 防止占据整批任务。用户修改授权后可重新检查；达到有效期则不发送。重复提醒跳过错过的历史时段，不集中补发。站内通知是数据库内的交付结果，不调用外部通信工具，不声称已发送邮件。

## 与完整设计的差异

- 默认安装仍为 demo；V0.2 已验证用户指定网关的 LLM/ASR/TTS 真实连通性。真实情绪质量、300 样本 Macro-F1、人工适当率、模型费用和 P95 SLO 待专门评测。
- 候选提取是生成后、提交前的限时步骤，未采用独立任务队列；失败只丢弃本轮候选，不自动永久记住推断事实。
- 目前短期上下文为最近 20 条，没有 LLM 长摘要或独立情感记忆保留。情绪保留默认关闭且本版无开启入口。
- MCP 已支持配置驱动的只读工具发现与手动调用，但尚未提供真实 MCP 服务器。没有开放 LLM 自主外部操作或内置知识库上传。
- 录音转写与短回复朗读已实现；全双工实时语音、数字人、移动端独立客户端和桌面壳尚未实现。

## 技术参考

- [FastAPI lifespan](https://fastapi.tiangolo.com/advanced/events/)：用 lifespan 管理 HTTP 客户端生命周期。
- [FastAPI StreamingResponse](https://fastapi.tiangolo.com/advanced/stream-data/)：异步生成器流式返回。
- [pgvector 官方仓库](https://github.com/pgvector/pgvector)：PostgreSQL 中的向量列、距离查询与索引。
