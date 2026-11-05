# 贡献指南

感谢关注本项目。提交代码前请先阅读以下约定。

## 开始之前

- 本项目使用 Apache-2.0 许可，提交的代码将按同一许可发布。
- `admin/` 前端依赖商业组件库 `ele-admin-plus`，该库不包含在本仓库中。参与 `admin/` 开发前请自行安装并向原厂购买授权，本地开发环境需自备授权。
- 请勿在 issue、PR、提交信息、日志或截图中包含任何真实凭据、生产配置、内部地址和个人信息。

## 开发环境

| 组件 | 要求 |
| --- | --- |
| Go | 1.27 及以上 |
| Node.js | 20 及以上（admin、chat 均使用 Vite 8） |
| PostgreSQL | 后端主数据存储 |
| Redis | 缓存与会话 |
| WuKongIM | 即时通讯通道（可选，仅调试 IM 相关功能时需要） |
| Qdrant | 向量库（可选，仅调试知识库检索时需要） |

本地启动步骤见 [README.md](README.md) 的「快速开始」。

## 提交规范

1. 从主分支切出功能分支，命名如 `feat/xxx`、`fix/xxx`。
2. 提交信息使用中文或英文均可，请说明改动内容与原因，采用 `feat:`、`fix:`、`docs:`、`refactor:`、`chore:` 等前缀。
3. 一次提交只做一件事，避免把格式化、重命名与功能改动混在一起。
4. 提交前请自行运行：

```bash
cd server && go build ./... && go test ./...
cd admin && npm run lint
cd chat && npm run lint
```

5. 新增文件前确认没有把 `.env`、`config.toml`、日志、上传文件、本地数据或截图带进仓库。

## 提交 Pull Request

- 说明背景、改动点、验证方式；涉及界面的改动请附截图（隐去真实数据）。
- 涉及数据库结构变更时，请同步更新 `server/internal/migrate.go` 中的迁移逻辑。
- 涉及配置项变更时，请同步更新 `server/config.example.toml` 或对应的 `.env.example`，值一律留空或使用占位符。
- 保持改动聚焦，便于评审。

## 代码风格

- Go：遵循 `gofmt`，错误需显式处理，避免静默忽略。
- TypeScript / Vue：沿用现有 ESLint 与 Prettier 配置，组件使用 `<script setup lang="ts">`，缩进与命名与相邻文件保持一致。
- 注释与文档使用中文，术语保持统一。

## 安全问题

请勿通过公开 issue 报告安全漏洞，见 [SECURITY.md](SECURITY.md)。
