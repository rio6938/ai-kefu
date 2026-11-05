# 接口权限说明

后台接口采用「登录 + 按钮权限」两级校验：登录由 `middleware.Auth()` 负责，
接口权限由 `middleware.Permission(authority)` 负责。按钮权限与菜单共用
`sys_menu` / `sys_role_menu` 两份数据，按钮就是 `menu_type = 1` 的菜单节点，
不额外引入权限表。

## 一、判定流程

```
Logger → Recovery → OperationRecord → Auth(登录) → Permission(权限) → handler
```

`Permission` 的判定顺序：

1. 上下文取不到登录用户 → `401`「请先登录」（说明路由漏挂 `Auth()`）；
2. 角色中包含 `admin`（`sys_role.role_code`）→ 直接放行，不查菜单；
3. 用户可见菜单（含按钮）上的 `authority` 命中要求的权限标识 → 放行；
4. 未命中 → `403`「无权限操作」，同时写入 gin 错误，操作日志记为失败；
5. 加载权限出错 → `500`「校验权限失败」，失败即拒绝，不降级放行。

超级管理员的权限来自「返回全部菜单」分支，不依赖 `sys_role_menu`，
所以新增按钮节点后无需给管理员角色手工授权。前端 `use-permission.ts`
没有通配符（`SUPER_ADMIN_AUTH` 为 `null`），管理员能看到全部按钮同样依赖
`/auth/user` 返回的完整 `authorities`。

## 二、只校验登录的接口（白名单）

这些接口是登录链路、页面内的基础数据或基础能力，挂权限码会导致功能不可用：

| 接口 | 原因 |
| --- | --- |
| `POST /admin/login`、`GET /admin/captcha` | 登录前调用 |
| `DELETE /admin/logout`、`GET /admin/auth/user` | 退出登录、读取自身信息与菜单 |
| `POST /file/upload`、`POST /file/upload/base64` | 换头像、富文本插图、附件等页面内基础能力 |
| `GET /file/raw`、`GET /file/thumbnail`、`GET /file/download` | 头像、图片预览、附件下载 |
| `GET /admin/role`（不分页） | 用户编辑弹窗的角色下拉 |
| `GET /admin/dictionary`（不分页） | 各处字典下拉 |
| `GET /admin/dictionary-data`（不分页） | 字典标签、字典下拉（如性别选择） |

`/user/**` 是前台用户接口，走 `middleware.UserAuth()` 与前台 token，
不参与本套权限校验。

其它 `/admin/**`、`/file/**` 接口一律校验权限。

## 三、接口与权限标识对照

### 系统管理

| 接口 | 权限标识 |
| --- | --- |
| `GET /admin/user/page`、`GET /admin/user`、`GET /admin/user/:id` | `sys:user:list` |
| `GET /admin/user/existence` | `sys:user:list` |
| `POST /admin/user` | `sys:user:save` |
| `PUT /admin/user`、`PUT /admin/user/status`、`PUT /admin/user/password` | `sys:user:update` |
| `DELETE /admin/user/batch`、`DELETE /admin/user/:id` | `sys:user:remove` |
| `GET /admin/role/page` | `sys:role:list` |
| `POST /admin/role` | `sys:role:save` |
| `PUT /admin/role` | `sys:role:update` |
| `DELETE /admin/role/batch`、`DELETE /admin/role/:id` | `sys:role:remove` |
| `GET /admin/role-menu/:roleId` | `sys:role:list` |
| `PUT /admin/role-menu/:roleId` | `sys:role:update` |
| `GET /admin/menu/page`、`GET /admin/menu` | `sys:menu:list` |
| `POST /admin/menu` | `sys:menu:save` |
| `PUT /admin/menu` | `sys:menu:update` |
| `DELETE /admin/menu/:id` | `sys:menu:remove` |
| `GET /admin/dictionary/page` | `sys:dict:list` |
| `POST /admin/dictionary` | `sys:dict:save` |
| `PUT /admin/dictionary` | `sys:dict:update` |
| `DELETE /admin/dictionary/:id` | `sys:dict:remove` |
| `GET /admin/dictionary-data/page` | `sys:dict:list` |
| `POST /admin/dictionary-data` | `sys:dict:save` |
| `PUT /admin/dictionary-data` | `sys:dict:update` |
| `DELETE /admin/dictionary-data/batch`、`DELETE /admin/dictionary-data/:id` | `sys:dict:remove` |
| `GET /admin/config` | `sys:config:list` |
| `PUT /admin/config/group` | `sys:config:update` |
| `PUT /admin/auth/user` | `sys:auth:user` |
| `PUT /admin/auth/password` | `sys:auth:password` |
| `GET /admin/login-record/page`、`GET /admin/login-record` | `sys:login-record:list` |
| `DELETE /admin/login-record/batch`、`DELETE /admin/login-record/:id` | `sys:login-record:remove` |
| `GET /admin/operation-record/page`、`GET /admin/operation-record` | `sys:operation-record:list` |
| `DELETE /admin/operation-record/batch`、`DELETE /admin/operation-record/:id` | `sys:operation-record:remove` |
| `GET /admin/user-file/page`、`GET /admin/user-file` | `sys:user-file:list` |
| `POST /admin/user-file` | `sys:user-file:save` |
| `PUT /admin/user-file` | `sys:user-file:update` |
| `DELETE /admin/user-file/batch`、`DELETE /admin/user-file/:id` | `sys:user-file:remove` |

说明：

- `sys:user-file:*` 目前没有对应的前端页面，因此没有在菜单树里建按钮节点，
  非管理员无法使用这几个接口；页面落地时补节点即可。
- `PUT /admin/config/group` 只有一个接口，密钥分组（`sys:config:secret`）
  本期不单独校验，统一要求 `sys:config:update`。
- 取消「查询用户」按钮会让用户编辑弹窗的账号唯一性校验（`/admin/user/existence`）
  返回 403，该角色将无法新增或编辑用户。

### 文件

| 接口 | 权限标识 |
| --- | --- |
| `GET /file/page`、`GET /file/list` | `sys:file:list` |
| `DELETE /file/remove/batch`、`DELETE /file/remove/:id` | `sys:file:remove` |

### 前台用户（后台侧）

| 接口 | 权限标识 |
| --- | --- |
| `GET /admin/user-center/page`、`GET /admin/user-center/existence`、`GET /admin/user-center/:id` | `sys:user-center:list` |
| `POST /admin/user-center` | `sys:user-center:save` |
| `PUT /admin/user-center`、`PUT /admin/user-center/status`、`PUT /admin/user-center/password` | `sys:user-center:update` |
| `DELETE /admin/user-center/batch`、`DELETE /admin/user-center/:id` | `sys:user-center:remove` |
| `GET /admin/user-login-record/page`、`GET /admin/user-login-record` | `user:login-record:list` |
| `DELETE /admin/user-login-record/batch`、`DELETE /admin/user-login-record/:id` | `user:login-record:remove` |
| `GET /admin/user-operation-record/page`、`GET /admin/user-operation-record` | `user:operation-record:list` |
| `DELETE /admin/user-operation-record/batch`、`DELETE /admin/user-operation-record/:id` | `user:operation-record:remove` |

## 四、授权数据

- 角色授权入口是「角色管理 → 分配权限」弹窗（`role-auth.vue`），
  数据来自 `GET /admin/role-menu/:roleId`，保存时整份覆盖 `sys_role_menu`；
- 弹窗里勾选菜单会**级联勾选其下全部按钮**。需要只读角色时，勾完菜单再取消
  保存、删除、修改，保留查询即可；
- 迁移 `Migrate()` 会幂等补齐以下按钮节点，并在首次创建时回填历史角色的授权
  （已授权菜单下的按钮一并授权，同时补齐祖先菜单，避免出现有按钮却看不到菜单）：
  - `sys:login-record:remove` / `sys:operation-record:remove`
  - `user:login-record:remove` / `user:operation-record:remove`
- 回填只在按钮节点首次创建时执行一次，避免每次启动把管理员手工取消的授权补回来；
- 管理员角色跳过回填，其权限始终为全部菜单。

## 五、运维须知

1. 新建角色默认没有任何权限，必须在「分配权限」里勾选后才能登录使用；
2. 权限修改对接口**立即生效**；侧边栏与已打开页面需要刷新才会更新；
3. 管理员账号与管理员角色受保护（不可删除角色、不可冻结、不可移除角色），
   永远拥有全部权限；
4. 被拒绝的写请求会在操作日志里记为失败，错误信息为「无权限操作」。

## 六、验收用例

| 场景 | 期望 |
| --- | --- |
| 无 token 调用受保护接口 | `401`「请先登录」 |
| 普通角色调用未授权接口 | `403`「无权限操作」，操作日志有失败记录 |
| 同一角色授权后立即重试（不重新登录） | `200` |
| 管理员账号调用任意接口 | `200` |
| 白名单接口仅带 token | `200` |
| 只有用户管理权限的角色打开用户编辑弹窗 | 角色下拉有数据、性别可选、保存成功 |
| 同上，换头像、富文本插图、下载附件 | `200` |
| 只授权登录日志菜单的角色 | 侧边栏只有登录日志，其它接口 `403` |
| 授权弹窗不做修改直接保存 | 角色权限不丢、菜单不消失 |
