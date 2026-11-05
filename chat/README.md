# AI 智能客服 · 网页聊天窗

客户侧的独立聊天窗，供门店/官网通过 iframe 嵌入。与 `admin/` 完全独立：不共享源码目录、不互相 import，只共用同一套接口与 IM 消息协议。

## 技术栈

- Vue 3 + TypeScript + Vite
- Element Plus（全局安装）
- `vue-i18n@^11`：聊天面板的中英文案（`legacy: false` + Composition API，缺失 key 回退简体中文）
- `easyjssdk@2.0.5`：连接 WuKongIM，收发消息与接收 AI 流式事件

## 开发

```bash
cd chat
npm install
npm run dev
```

启动后访问 `http://localhost:5174/?channel=web-default`。

`vite.config.ts` 已把 `/kefu`、`/v1` 代理到 `VITE_API_PROXY_URL`（默认 `http://127.0.0.1:8080`），后端按 `server/config.toml` 启动即可联调。

## 构建与部署

```bash
cd chat
npm install
npm run build      # 先 vue-tsc 类型检查, 再 vite build, 产物在 chat/dist
```

部署方式二选一：

1. **同源反代（推荐）**：把 `chat/dist` 部署到 `https://www.example.com/chat/`，nginx 把 `/kefu`、`/v1` 反代到后端。此时 `.env` 的 `VITE_API_URL` 留空即可，后端也不需要开启跨域。
2. **跨域部署**：聊天窗与接口不同域时，`.env` 配 `VITE_API_URL=https://api.example.com`，同时在后端 `config.toml` 的 `[cors] allow_origins` 里放行聊天窗域名（后端只对 `/kefu`、`/v1` 开放跨域，`/admin` 不开放）。

## 嵌入方式

### 推荐：悬浮客服脚本（类 Crisp）

宿主页面一行引入 `chat/public/widget.js`（构建后位于 `chat/dist/widget.js`，单独部署到静态目录或直接随 `dist` 一起发布），右下角会出现客服气泡，点开即聊天面板：

```html
<script src="https://www.example.com/widget.js" data-channel="web-default"></script>
```

可用属性：

| 属性 | 说明 |
| --- | --- |
| `data-channel` | 渠道标识，默认 `web-default` |
| `data-lang` | 语言，`zh-CN` / `en`，不填按 `KEFU_WIDGET_CONFIG.locale` → `html lang` → 浏览器语言 |
| `data-position` | `right`（默认）或 `left` |
| `data-color` | 主色，不填取渠道 `themeColor`，再兜底 `#409eff` |
| `data-welcome` | `0` 表示不展示欢迎气泡，默认展示一次 |
| `data-hidden` | `1` 表示初始隐藏，之后用 `setVisible(true)` 显示 |
| `data-chat-url` / `data-api` | 面板地址与接口地址，不填时从 `KEFU_WIDGET_CONFIG` 或渠道公开配置读取 |

宿主内联配置（可选）：

```html
<script>
  window.KEFU_WIDGET_CONFIG = {
    locale: 'zh-CN',
    chatUrl: 'https://www.example.com/chat/',
    apiBase: ''
  };
</script>
```

对外 API 与 `$crisp` 对齐，方便迁移：

```js
window.KeFuWidget.open();       // 展开面板
window.KeFuWidget.close();      // 收起面板
window.KeFuWidget.toggle();     // 切换开合
window.KeFuWidget.setLocale('en');
window.KeFuWidget.setVisible(true);
window.KeFuWidget.setUnread(3);  // 未读红点，>99 显示 99+
window.KeFuWidget.identify({ nickname: '张三', email: 'a@b.com' });
```

说明：

- 脚本零依赖、无构建，样式全部挂在 `#kefu-widget-root` 下，不读写宿主 `body` 样式，不改宿主其它元素。
- 同一页面重复引入只渲染一次（`window.__kefuWidgetLoaded` + 容器 id 双重判断）。
- `identify` 只接受 `nickname` / `email`，没有业务 `userId` 入口。
- 子 → 父消息：`kefu:ready` / `kefu:unread` / `kefu:close` / `kefu:lang`；父 → 子：`kefu:open` / `kefu:close` / `kefu:lang` / `kefu:identify`，`targetOrigin` 固定为面板域名。
- 面板地址取配置中心 `kefu.web.chat_url`，脚本地址取 `kefu.web.widget_url`，后台「渠道管理 → 嵌入代码」两个版本都会给出。

### 兼容：整块 iframe

```html
<iframe
  src="https://www.example.com/chat/?channel=web-default"
  style="width: 420px; height: 640px; border: none"
  title="在线客服"
></iframe>
```

被悬浮脚本打开时面板地址会自动带上 `?embed=widget&lang=xx`，此时页面不渲染整页外壳、无滚动条、高度铺满父容器。

## 目录结构

```
src/
├── main.ts               入口, 全局安装 vue-i18n 与 Element Plus
├── App.vue               用 ElConfigProvider 按当前语言下发 Element Plus 语言包, 只渲染聊天窗
├── views/ChatWindow.vue  聊天窗主体: 顶部信息 + 消息列表 + 输入区
├── i18n/
│   ├── index.ts          createI18n / 语言优先级解析 / setLocale / 组件外的 t()
│   └── lang/             zh-CN.ts / en.ts, key 前缀 chat.*
├── components/
│   ├── chat-header.vue   标题 / 客服名 / AI 标识 / 会话状态 / 语言下拉 / 修改资料入口
│   ├── prechat-form.vue  咨询前预填昵称与邮箱（可跳过）
│   ├── message-bubble.vue 气泡内容, 文本 / 图片缩略图 / 附件下载, AI 消息带「AI 助手」标识
│   ├── quick-replies.vue 输入框上方的快捷问题胶囊, 点击直接发送
│   └── composer.vue      输入框 + 快捷问题 + 附件上传 + 发送
├── composables/
│   ├── use-bootstrap.ts  读 ?channel → 渠道配置 → bootstrap → 落 visitorKey / 访客资料
│   ├── use-messages.ts   消息列表 / 历史消息 / 去重追加 / 上传占位替换
│   └── use-stream.ts     CustomEvent 增量合并(event_id 去重, 按 client_msg_no 归并)
├── im/client.ts          easyjssdk 封装: 连接 / 发送 / 重连 / 销毁
├── api/                  channel / session / quick-reply / file / types
├── utils/request.ts      与 admin 一致: code === 0 判成功
└── styles/               聊天窗全局样式与主题变量
public/
└── widget.js             悬浮客服脚本, 零依赖原生实现, 自带极简文案表 widget.*
```

## 国际化

两层各自负责自己的语言，任何一层都不依赖另一层加载成功：

| 层 | 实现 | 文案位置 | 语言来源 |
| --- | --- | --- | --- |
| 悬浮脚本 `widget.js` | 零依赖原生，内置 `MESSAGES` 表 | `widget.*`（气泡 `aria-label`、欢迎气泡、问候语） | `data-lang` → `KEFU_WIDGET_CONFIG.locale` → `html lang` → 浏览器语言 → `zh-CN` |
| 聊天面板 `chat/` | `vue-i18n@^11`（`legacy: false`） | `chat.*`（`chat/src/i18n/lang/*.ts`） | `?lang=` → `localStorage.kefuLang` → `html lang` → 浏览器语言 → `zh-CN` |

- 首批语言 `zh-CN` / `en`，`zh-TW` 为预留代码，暂无译文时复用简体中文；未知语言一律回退 `zh-CN`，不会出现空白。
- 时间段问候语（早/午/晚/夜深）由面板与脚本**各自**按本地时间计算，两边文案分段一致。
- 面板顶部语言下拉（中文 / English）切换时：更新 `vue-i18n` locale、`document.documentElement.lang`、`localStorage.kefuLang`，并向父窗口发 `kefu:lang`，脚本据此重绘气泡与欢迎语，**不刷新页面、不断 IM 连接**。
- 脚本切换语言时通过 `?lang=`（未加载完）或 `kefu:lang`（已加载）同步给面板。
- 不翻译的内容：客服昵称、知识库/模型返回的业务内容、用户自己发送的消息。

## 关键行为

- 访客标识存在 `localStorage.kefuVisitorKey`，同一浏览器重复进入复用同一个访客与进行中的会话。
- 访客资料存在 `localStorage.kefuVisitorProfile`（`{nickname, email}`），首次进入先展示预填卡片，两者都可留空；点「跳过」不写存储。
- 预填卡片只在没有 `kefuVisitorKey` 且没有本地资料时出现；面板右上角头像菜单可随时「修改资料」。
- 客户消息通过 IM SDK 直接发送（`channel_id = session_{sessionId}`），服务端 webhook 负责落库，因此关闭页面再打开不会丢消息。
- AI 回复以 `stream.open` / `stream.delta` / `stream.finish` 事件到达，增量按 `event_id` 去重、按 `client_msg_no` 归并到同一个气泡，`stream.finish` 带完整快照，气泡一定收敛到终态。
- 收到客服或 AI 消息后自动刷新会话详情与历史消息；`pagehide` 时销毁 IM 连接。

## 快捷回复与附件

- 访客快捷问题：进入后拉 `GET /kefu/quick-reply?channelCode=xxx`，输入框上方展示可横向滚动的胶囊，点击**直接发送** `content`；无数据时整块不渲染。
- 访客附件：输入框工具栏「附件」按钮，后缀与大小在选文件时先按渠道公开配置（`fileEnabled` / `fileMaxSizeMb` / `fileAllowedExt`）拦截，再走 `POST /kefu/file/upload`（multipart）上传，成功后发送 `type = image | file` 的 IM 消息，`content` 存附件地址、`meta` 存 `{fileId,name,size,contentType}`。
- 附件消息渲染：`image` 用 `ElImage` 缩略图（点击预览），`file` 展示文件名 + 大小 + 下载链接；历史消息刷新后靠 `payload.meta` 恢复。
- 上传中先上一个 `status: 'uploading'` 的占位气泡，失败替换为可读的错误提示。
- 访客附件读取走 `GET /kefu/file/raw/*path`（免登录，随机文件名 + 后缀白名单 + 防目录穿越），不做逐条鉴权；单文件大小、允许后缀、单 IP 每日累计大小三项都在配置中心可改，改完立即生效。
