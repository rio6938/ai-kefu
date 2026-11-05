# AI 智能客服（ai-kefu）

一个可自托管的 AI 智能客服系统：后台管理端 + Go 后端 + 访客网页聊天窗，支持多渠道接入、人工坐席、知识库与 AI 自动回复。

## 架构

| 目录 | 说明 | 技术栈 |
| --- | --- | --- |
| `admin/` | 后台管理前端，坐席工作台、渠道/知识库/模型/提示词配置、数据看板。仓库内为**构建好的静态产物**，源码不含在内 | Vue 3 + TypeScript + Vite + Element Plus |
| `server/` | 后端服务，统一的 HTTP API、鉴权、业务逻辑、AI 编排与数据迁移 | Go 1.27 + Gin + GORM + PostgreSQL + Redis |
| `chat/` | 访客端网页聊天窗，可独立构建、独立部署 | Vue 3 + TypeScript + Vite |

后端依赖的外部组件：

- **PostgreSQL**：主数据存储
- **Redis**：缓存、会话与队列
- **WuKongIM**：即时通讯通道（登录后向客户端下发 gateway 地址）
- **Qdrant**：向量库，用于知识库检索（RAG）

## 功能概览

- 多渠道接入：网页聊天、邮箱渠道（收信/发信/会话归并）、渠道级配置
- AI 自动回复：模型与提示词配置、知识库检索增强、AI 草稿供坐席确认
- 人工坐席：排队与转人工、坐席状态、快捷回复、会话标签、留言与评价
- 运营能力：会话分析、消息记录、文件与知识库管理

## 快速开始

### 1. 后端

```bash
cd server
go mod download
go run . -gen-config config.example.toml   # 如需重新生成配置示例
cp config.example.toml config.toml         # 按实际环境修改（数据库、Redis、IM、Qdrant 等）
go run . -c config.toml
```

默认监听 `:18080`。`config.toml` 内含真实凭据，已在 `.gitignore` 中排除，请勿提交。

### 2. 后台管理端

`admin/` 已是构建好的静态站点，不需要安装依赖或编译，把整个目录交给静态服务器即可。接口前缀在构建时已固定为 `/api`，需要一个同源反向代理指向后端：

```nginx
server {
    listen 80;
    root /path/to/ai-kefu/admin;
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:18080/api/;
    }

    # 单页应用，history 路由需回退到 index.html
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

> **关于后台源码：** `admin/` 只提供构建产物，不含源码。后台前端依赖商业组件库 EleAdminPlus，其授权条款不允许公开源代码，因此本仓库不收录该库及依赖它的业务源码。
> 产物中不含任何真实密钥（地图 key 等构建时置空），接口地址与显示名称在构建时写入；如需二次开发或修改这些值，请自行取得该库授权后重建工程。

### 3. 访客端聊天窗

```bash
cd chat
npm install
cp .env.example .env
npm run dev             # 默认 5174 端口，把 /kefu、/v1 代理到后端
```

聊天窗构建产物为纯静态文件，可直接放入任意静态服务器或由后端同源反代。

## 环境变量

### admin（构建时变量，非运行时配置）

`admin/` 是产物目录，仓库里没有 `admin/.env`，以下变量在构建时就被写死了，改值需要重新构建：

| 变量 | 说明 | 本产物的取值 |
| --- | --- | --- |
| `VITE_API_URL` | 前端请求的接口前缀 | `/api` |
| `VITE_API_PROXY_URL` | 开发环境接口代理目标，生产不使用 | 空 |
| `VITE_MAP_KEY` | 高德地图 JS API key，需自行申请 | 空 |
| `VITE_MAP_SECRET` | 高德地图安全密钥 | 空 |
| `VITE_DEV_TOOLS` | 是否启用 Vue DevTools 插件 | `false` |

### chat/.env

| 变量 | 说明 |
| --- | --- |
| `VITE_APP_NAME` | 聊天窗显示名称 |
| `VITE_API_URL` | 接口地址前缀，与后端同源反代时留空 |
| `VITE_API_PROXY_URL` | 开发环境接口代理目标，生产不使用 |

`chat/.env.example` 只保留键名与占位说明，所有值均为空，不包含任何真实密钥或本机地址。

### server/config.toml

以 `server/config.example.toml` 为模板，主要包含：监听地址与日志、PostgreSQL DSN、Redis、JWT 密钥、验证码、安全配置密钥、WuKongIM 服务地址与回调密钥、Qdrant 地址与 `api_key`、跨域白名单。示例文件中所有密钥字段均为空字符串，需要自行生成并填写。

## 安全提示

- 仓库中不含任何真实凭据、生产配置或运行数据；`.env`、`config.toml`、日志、上传文件、数据库文件均在忽略规则内。
- 部署时请为 `jwt.secret`、`security.config_key`、`wukongim.webhook_token`、`qdrant.api_key` 等字段生成独立随机值，不要使用示例或默认值。
- 后台登录页带有预填的演示账号，后端也有默认的重置密码值；部署到公网前请先修改管理员密码，并在后端调整默认重置值。
- 发现安全问题请参考 [SECURITY.md](SECURITY.md)，不要公开提交 issue。

## 参与贡献

见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 发布，Copyright 2026 rio6938。

`admin/` 下的前端产物由依赖商业组件库 EleAdminPlus 的源码构建而成。该组件库为第三方闭源商业软件，不包含在本仓库内，其授权与分发条件由原厂提供，不适用本项目的 Apache-2.0 许可；使用或再分发前请自行向原厂取得授权。
