# 项目协作约定

开发前阅读根目录代码规范和 docs/implementation.md。按 services/api、services/ai、apps/web 分工，领域层不得依赖 HTTP 或数据库实现。Python 使用 services/ai/.venv，不向全局环境安装项目依赖。

新增业务必须维护 packages/contracts、核心规则测试和 README。运行 scripts/check.ps1。禁止提交 .env、真实聊天和密钥。模型结果是不可信数据，必须 Schema 校验；所有跨用户访问由服务端校验。记忆逐条确认，模拟状态不能由 LLM 修改，停止交互优先于人格。
