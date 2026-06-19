# Changelog

## [Unreleased]

### Changed

- 增加配置启动校验，要求 `server.jwt_secret` 使用至少 32 位强随机字符串，并要求 `server.jwt_expire_hours` 大于 0。
- 默认关闭内置管理员/测试账号补种，改由 `server.seed_default_accounts` 显式控制。
- 同步部署文档中的前端目录与技术栈说明，统一为 `web/app` 和 React 19 + Vite。

### Fixed

- 从 Git 跟踪中移除已提交的构建产物和运行期导出 CSV，并补充 `.gitignore` / `.dockerignore` 规则，避免后续继续提交二进制、上传文件和导出数据。
- 修复前端 lint 中的确定性错误，并显式关闭当前项目尚未适配的 React Hooks 严格规则。
