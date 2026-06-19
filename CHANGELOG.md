# Changelog

## [Unreleased]

### Changed

- 增加配置启动校验，要求 `server.jwt_secret` 使用至少 32 位强随机字符串，并要求 `server.jwt_expire_hours` 大于 0。
- 默认关闭内置管理员/测试账号补种，改由 `server.seed_default_accounts` 显式控制。
- 默认关闭示例渠道补种，避免生产空库自动创建带占位 key 的启用渠道。
- 同步部署文档中的前端目录与技术栈说明，统一为 `web/app` 和 React 19 + Vite。
- 前端统一使用 npm lockfile，新增 CI、Dependabot 和 `npm run audit:security`。
- 构建工具链固定到 Go 1.26.4，并升级 `x/net`、`x/crypto`、`mapstructure` 等依赖以通过 `govulncheck`。

### Fixed

- 修复媒体任务保留字段注入、上传类型校验、导出文件公开暴露、上游日志敏感信息入库、API Key 缓存失效、PayApply 并发回调幂等等安全问题。
- 修复页眉/页脚 HTML、Markdown 链接、上传预检、导出下载和手动调账的前端安全与资金语义问题。
- 修复 PostgreSQL 迁移脚本语法和在线索引方式，并更新批量迁移命令。
- 修复 Docker nginx `/health` 与部署验收命令不一致的问题。
- 从 Git 跟踪中移除已提交的构建产物和运行期导出 CSV，并补充 `.gitignore` / `.dockerignore` 规则，避免后续继续提交二进制、上传文件和导出数据。
- 修复前端 lint 中的确定性错误，并显式关闭当前项目尚未适配的 React Hooks 严格规则。
