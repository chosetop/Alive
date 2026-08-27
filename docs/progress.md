# Alive 施工进度

最后更新：2026-08-26（Draft Autosave Foundation 浏览器验收 preflight 已记录，见第 14 节）

本文档记录已完成的内容、当前状态和待办项。设计依据见 `architecture.md`，认证方案见 `stage-auth-plan.md`。

## 1. 阶段总览

| 阶段 | 内容 | 状态 |
|---|---|---|
| Phase 1 | 架构设计文档 | 完成 |
| Phase 2 | 后端基础设施初始化 | 完成 |
| Stage 1 计划 | users + auth 实施方案 | 完成 |
| 1A | migration 000002 + sqlc 基础设施 | 完成 |
| 1B | auth 领域层（model / password / token / repository） | 完成 |
| 1C-1 | auth Service | 完成 |
| 1C-2 | session Cookie | 完成 |
| 1C-3 | Handler + Middleware + 路由装配 | 完成 |
| 1C-4 | 登录限流 | 完成 |
| 1C-5 | `user:create` 与 `session:prune` CLI | 完成 |
| — | 集成测试库隔离 | 完成 |
| — | README / Makefile / architecture.md 对齐 | 完成 |
| 2A | migration 000003 + `sql/queries/entry.sql` | 完成 |
| 2B | `internal/entry` 领域层 | 完成 |
| 2C | `internal/entryhttp` + 路由注册 | 完成 |
| — | 测试库并行隔离重构（`internal/dbtest`） | 完成 |
| 2D | Entry 的改 / 删 / 状态迁移 + Admin 读取路径 | 完成 |
| 2E | `categories` + `unlisted` 独立查询 | 完成 |
| 3A | `admin/` 工程初始化 + 认证闭环 | 完成 |
| — | 开发库补 migration 000004 | 完成 |
| 3B | `admin/` 内容管理（分类 + 文章列表 + Milkdown 编辑器） | 完成 |
| 4A-0 | `frontend/` 工程初始化 + SSR 认证骨架 | 完成 |
| 4A | `frontend/` 前台页面（列表 / 详情 / 分类）+ 设计系统 | 完成 |

**Stage 1（users + auth）与 Stage 2（Entry + Categories）的后端 API 均已完成**，共 19 个业务端点加 2 个探针。

**`admin/` 已初始化并跑通认证闭环**（3A，2026-08-25，见第 10 节）：登录、刷新恢复、登出、会话中途失效四条路径都过了浏览器实测。文章 CRUD、编辑器、分类管理不在 3A 范围内，见 3.6。

**`admin/` 内容管理已完成**（3B，2026-08-25，见第 12 节）：分类管理、文章列表、Milkdown 编辑器、状态迁移与删除全部接通。写路径逐条对着运行中的后端验过（见 12.4），但**界面本身没有点过**，环境里没有浏览器自动化。

**`frontend/` 前台页面已完成**（4A，2026-08-25，见第 13 节）：列表、详情、分类三个页面加一套设计 token 层与两套主题。Nuxt 版本是 **3.21.11**，非 Nuxt 4（见第 11 节）。SSR 输出、路由状态码、Markdown 注入向量、配色对比度都验过，**界面本身仍然没有在浏览器里看过**，环境里没有浏览器自动化。

修了一个后端行为：前台列表的排序从 `happened_at DESC NULLS LAST` 改为 `COALESCE(happened_at, published_at) DESC`（migration 000005），见 13.5。

**三端已实测可同时运行**（2026-08-25，见第 11 节）：后端 :8080、`admin/` :5173、`frontend/` :3000 全部返回 200，`owner` 登录闭环通过。

## 2. 当前可用的能力

后端可以启动、连接数据库、完成一次完整的登录会话生命周期。

```
POST /api/v1/auth/login     公开    200 + Set-Cookie / 401 / 400 / 429（限流）
POST /api/v1/auth/logout    公开    204，幂等
GET  /api/v1/me             需登录  200 / 401
POST   /api/v1/entries              需登录  201 / 400 / 401 / 409
PATCH  /api/v1/entries/:id          需登录  200 / 400 / 401 / 404 / 409
DELETE /api/v1/entries/:id          需登录  204 / 400 / 401 / 404（软删）
POST   /api/v1/entries/:id/publish  需登录  200 / 400 / 401 / 404
POST   /api/v1/entries/:id/unpublish 需登录 200 / 400 / 401 / 404
POST   /api/v1/entries/:id/archive  需登录  200 / 400 / 401 / 404
GET    /api/v1/entries              公开    200，分页
GET    /api/v1/entries/:slug        公开    200 / 404
GET    /api/v1/admin/entries        需登录  200，分页，含草稿，可按 status 筛
GET    /api/v1/admin/entries/:id    需登录  200 / 400 / 401 / 404，含草稿
POST   /api/v1/categories           需登录  201 / 400 / 401 / 409
PATCH  /api/v1/categories/:id       需登录  200 / 400 / 401 / 404 / 409
DELETE /api/v1/categories/:id       需登录  204 / 400 / 401 / 404（物理删，内容变未分类）
GET    /api/v1/categories           公开    200，含内容计数，不分页
GET    /api/v1/admin/categories     需登录  200，含时间戳，不含计数
GET    /api/v1/admin/categories/:id 需登录  200 / 400 / 401 / 404
GET    /health                      公开    存活探针，不碰数据库
GET    /health/ready                公开    就绪探针，ping 数据库
```

`GET /api/v1/entries` 接受 `?category=<slug>`。slug 解析不到是 404 而不是空列表：打错的链接和空分类是两回事。传空串等同于没传。

分类列表不分页。分类是导航，需要翻页的导航没人导航得了。

`GET /api/v1/entries/:slug` 能取到 `visibility=unlisted`，列表取不到。这不是访问控制：slug 可读因此可猜，真正不能给人看的内容用 `private`。

写路由按 id 寻址，前台详情按 slug 寻址。这不是不一致：读者靠 URL 认一篇文章，而编辑改的可能正是 slug 本身。

`PATCH` 拒收 `status` 字段并返回 400，而不是忽略它。忽略会让客户端拿到 200 却发现文章没发布，且永远找不到真正负责发布的端点。

已通过 13 项 curl 端到端验证，包括：密码错误与用户不存在的响应逐字节相同；篡改 cookie 末位字符被拒；登出后旧 cookie 失效；重复登出返回 204；数据库中 `token_hash` 与 `sha256(cookie 值)` 一致，即库里不存在可重放的凭证。

登录限流另有 5 项实机验证：第 6 次请求转 429 并带 `Retry-After`；被拒响应耗时 54µs 对比正常失败 110ms，即拒绝路径不做密码哈希；`Retry-After` 随时间递减，说明被拒请求不延长窗口；窗口关闭后额度重置；限流生效期间 `/api/v1/me` 与 logout 不受影响。

CLI 三个命令：

```
config:check     校验配置并打印（不含密钥，DSN 只报长度）
user:create      建账号，交互式两次读密码且不回显
session:prune    删除全部过期会话，幂等
```

`user:create` 拒绝 `--password`，因为 argv 会进 shell 历史与 `ps` 输出。`--password-stdin` 需显式声明，不做隐式回退。已用 `script(1)` 分配真实伪终端验证过：即使在捕获整条 pty 的情况下，密码也不出现在输出里。

### Entry 最初三个端点的实机验证（2C，2026-08-24）

在 `alive_test` 上起真实服务（8099 端口）跑完，用后即清，开发库未写入任何测试内容：

- 无 cookie 的 POST 得 401，请求没到达存储层
- 不带 `status` 建出来的是 draft，`published_at` 为 `null`；带 `status=published` 的当场盖上时间戳
- 前台列表只有那条 published，草稿不在其中，且列表响应不含 `content_md`
- **`GET /entries/secret-draft` 与 `GET /entries/never-used` 的响应体除 `request_id` 外逐字节相同**，都是 404 + `NOT_FOUND`
- 重复 slug 得 409 且 `fields.slug` 指名字段；`Kyoto Spring` 这类格式错误得 400
- `page_size=100000` 被夹到 50；`page=abc` 得 400 而不是静悄悄给第一页

可见性过滤另做过一次变异验证：把 `entrytest.Store` 里两处 `IsPubliclyReadable()` 判断打掉后，7 个子测试立刻转红，其中包括上面「草稿与不存在的 slug 响应必须相同」那条。这条证据说明该测试不是恒绿的装饰。

### 环境事实（新会话请先核对，勿凭记忆）

| 项 | 状态 |
|---|---|
| PostgreSQL | 18.6 运行中（brew services） |
| `alive`（开发库） | **版本 5**，表：`users` `sessions` `entries` `categories`。000005 是前台列表的表达式索引，见 13.5 |
| `alive_test`（测试库） | **版本 5**，表：同上。000005 已手动 apply（`make test-db-create` 只在建库时迁移，已有库需自己跑 `migrate ... up`） |
| 开发库账号 | `owner` / `alive-dev-password-2026`，display_name `P30 Huiwei`，role `owner`，id 105。**2026-08-25 再次验证仍有效**（登录 200 + `/me` 200）—— 需要登录时直接用它，不要另建账号 |
| 开发库数据量 | `entries` **11 行**（未软删）、`categories` **3 行**，其中 7 篇是 4A 造的验证数据，跨 2024–2026，含 1 篇故意留的草稿。**全都不是真实内容**，见 13.8 |
| 后端端口 | `.env` 里 `SERVER_PORT=8080`，实测监听 `127.0.0.1:8080`。`api.md` 与本文档下方提到的 8099 是 2C/2D/2E 用测试库跑验证时的临时端口，不是默认值 |
| Vite 端口 | `admin/` 固定 5173（`strictPort: true`），因为后端 `CORS_ALLOWED_ORIGINS` 白名单是精确匹配，带凭证的 API 不能用 `*` |
| Nuxt 端口 | `frontend/` 默认 3000，已在 `CORS_ALLOWED_ORIGINS` 白名单内 |
| `frontend/.env` | 已创建（2026-08-25），`NUXT_PUBLIC_API_BASE=http://localhost:8080`。三个 `.env` 均被各自 `.gitignore` 忽略 |
| `sqlc` / `golang-migrate` | 已装 |
| `GOPROXY` | 必须走 `https://goproxy.cn,direct`。`proxy.golang.org` 从此网络连不通，Makefile 已内置，但**在 make 之外手动跑 `go get` 需自己带上** |
| 端口检查 | 用 `lsof -iTCP:8080 -sTCP:LISTEN`。`lsof -ti:8080` 会匹配对端为 8080 的出站连接，据此杀进程会误杀无关程序（上个会话已误杀过一次 ToDesk） |

## 3. 尚未实现（按依赖顺序）

### 3.1 已解决：开发库补 migration（2026-08-25）

**已完成，此项不再阻塞。** 保留记录是因为症状具有误导性。

`alive` 曾停在版本 3 而 `alive_test` 已到 4。`categories` 表只存在于测试库，而 `entries` 的读取路径 JOIN 了它，于是**文章列表**也一起 500 —— 不只是分类接口：

```
GET /api/v1/entries    →  500
GET /api/v1/categories →  500
日志：list entries failed  error="entry: list public entries:
      ERROR: relation \"categories\" does not exist (SQLSTATE 42P01)"
```

已执行 `migrate ... up`，开发库现在是版本 4，两个端点都回 200。

成因：2E 的六个端点验证是在 `alive_test` 上做的，那次没碰开发库，于是版本差一直留到 3A 之后。**新会话遇到 `relation "..." does not exist` 先查库版本，不要去怀疑查询写错了。**

### 3.2 已解决：`admin/` 内容管理与 `frontend/` 前台页面

**两项都已完成。** `admin/` 内容管理见第 12 节（3B），`frontend/` 前台页面与设计系统见第 13 节（4A）。

**接口契约见 `docs/api.md`**，那份是按当前实现逐条核对过的。`architecture.md` 写的是设计意图与理由，两份的路径已于 2026-08-25 统一为 `/api/v1/*`，但只有 `api.md` 对着运行中的服务核对过字段与状态码，冲突以它为准。

前台还没有的东西：`sitemap.xml`、`feed.xml`、按类型的页面（书架、影单）、归档页。都不阻塞已有页面。

后端还剩的三件事：session 绝对过期、Argon2id 参数、trusted proxy，各自在 3.3 到 3.5。**其中 trusted proxy 是部署前必须做的**，见 3.5。

`tags`、`media`、`archives`、`site` 四组接口尚未开始，见 3.8。`tags` 会落在 `internal/taxonomy` 里 —— 那个包从一开始就是为「categories 今天、tags 以后」写的。

### 3.3 待办：绝对过期

当前只有 7 天滑动过期，没有绝对上限。一个每天都在用的 session 可以无限续下去。这是当时确认的行为，但值得记下来：如果 token 泄露且攻击者保持活跃，这个 session 永不自动失效。

补救方式是 `sessions` 表加 `absolute_expires_at`，登录时定死，续期不动它。需要新 migration。

### 3.4 待办：Argon2id 参数

实测 **212ms/次**，参数 m=64MiB / t=3 / p=2。

含义：登录接口每请求占 212ms CPU。单 IP 在 5 次/分钟限流下最多消耗 1 秒 CPU/分钟，可接受；但分布式来源下这是放大器——攻击者发一个 HTTP 包，服务端花 212ms。

OWASP 推荐 m=46MiB / t=1 / p=1，约 50ms。`t=3` 是主要成本来源，降到 `t=1` 约 70ms，仍在推荐线上。个人站登录频率极低，倾向保持不变。**未决。**

### 3.5 待办：`trusted proxy` 与真实客户端 IP

**这一项现在有了实际后果，不再只是理论问题。** 限流已上线，而它按 `c.ClientIP()` 分桶。

`router.go` 目前 `SetTrustedProxies(nil)`，所以 `c.ClientIP()` 返回 socket 自身地址。本机直连时它是真实来源，正确。一旦前面放了 Caddy，所有请求的 `ClientIP` 都变成代理那一个地址，全部落进同一个桶：**限流从「每 IP 5 次/分钟」退化为「整站 5 次/分钟」，任何一个人打满额度就把站主一起锁在门外。**

部署前必须把真实代理地址填进 `SetTrustedProxies`，让 `ClientIP()` 读 `X-Forwarded-For`。这一点已写进 `middleware/ratelimit.go` 中 `Middleware` 的注释，就在读 `ClientIP` 的那几行旁边。

注意顺序：先配 `SetTrustedProxies` 再上代理。反过来做，中间那段时间限流是全局的。

### 3.6 已完成：`admin/` 内容管理（3B，2026-08-25）

四步都已落地，详见第 12 节。仍未做的是**界面实测**：所有验证都是对着后端发请求做的，没有人点过按钮。

3B 之后 `admin/` 剩下的缺口：媒体上传（依赖 OSS，未开始）、批量操作、搜索。

原先列在这里的「`private` 可见性在前台的实际拦截」已在 4A 验过：`/draft-not-ready` 与 `/nonexistent-slug` 的响应体逐字节相同，见 13.7。

### 3.7 待办：`Retry-After` 在开发环境读不到

429 响应确实带 `Retry-After`（3A 实测 43 秒），但 `internal/config/config.go:249` 把 `ExposedHeaders` 硬编码成 `["X-Request-ID"]`，跨域 `fetch` **读不到未 expose 的头**。所以开发环境下 admin 只能显示兜底文案「尝试次数过多，请稍后重试。」，拿不到具体秒数。

生产环境 admin 与 API 同域，同源请求不受 expose 限制，秒数会正常出现。所以这不是 bug，只是开发与生产的行为差异，代码里按可选处理并注释了原因。

要让开发环境也显示秒数，就给 `ExposedHeaders` 加 `"Retry-After"`。这是后端改动，3A 没做。

### 3.8 后续阶段

Media 与对象存储（OSS）、Tag、Markdown 渲染（前端负责，后端只存源文本）、Public API 与 Admin API 的分离、SEO 相关的服务端渲染、Docker、CI/CD。均未开始。

## 4. 悬而未决的产品问题

1. **服务器位置** — 影响延迟与备案。部署前定。
2. **admin 的部署形态** — 后端 `.env` 注释里写的是「admin 从站点自有域名的 `/admin` 提供，浏览器调 `/api` 是同源，生产环境 `CORS_ALLOWED_ORIGINS` 应为空」。3A 是按这个前提做的（也正因如此 `Retry-After` 在生产环境能读到，见 3.7）。如果改成独立子域，那条前提和 CORS 配置都要重新算。

已定的三条，留在这里是因为「为什么是它」比「是它」更容易被重新提出：

- **第一批 `type` 取值** — 六值一次定齐，见第 7 节。
- **`frontend/` 的框架** — **Nuxt**，2026-08-25 确认，见下。
- **admin 的编辑器** — **Milkdown**，2026-08-25 确认，见下。

### admin 的编辑器是 Milkdown（2026-08-25 确认）

三个候选：朴素 textarea + 预览、CodeMirror 6、Milkdown。**已定 Milkdown**，版本 7.22.1。

选它意味着**所见即所得**：`##` 打完当场变成标题，而不是在左边写源码右边看预览。代价是三个候选里依赖最重的一个，且粘贴外部内容时容易带进意外结构。

这个选型的影响面比它看起来小：后端只存 Markdown 源文本（`content_md`），渲染由前台在 SSR 阶段做，所以**它既不影响后端也不影响前台**，只决定站主在 admin 里的编辑体验。换掉的成本是重写 admin 的一个视图，不涉及数据迁移。

已核实 `@milkdown/*` 7.22.1 的 peer dependency 是 `vue: ^3.0.0`，admin 装的是 Vue 3.5.41，兼容，不需要降级。3A 刻意没引入任何 UI 库，所以这次是在干净的地基上装。

### `frontend/` 是 Nuxt，不是 Astro（2026-08-25 确认）

3A 期间发现两份文档对不上：`progress.md` 有一处写 Astro，而 `architecture.md` 技术栈表、架构图、部署图、部署说明共四处写 Nuxt，`backend/.env.example` 的 CORS 白名单也早已为 Nuxt 的 3000 端口开着。

已确认 **Nuxt 是真实决策**，那一处 Astro 是笔误，已改。

这条值得留下记录，因为两个框架的差别不只是名字：Astro 默认零 JS 静态输出，Nuxt 是带 Node 运行时的 SSR。选 Nuxt 意味着部署时 `frontend/` 需要一个 Node 进程（部署图里就是这么画的：Caddy 反代到 Nuxt、admin 静态文件、Go 二进制三个上游），而不是一堆丢给 CDN 的静态文件。「frontend 负责 Markdown 渲染」也因此是在 SSR 阶段做，不是构建期做完就固定了。

选它的理由见 `architecture.md` 技术栈表：SEO 重要，而 Vue 生态里成熟的 SSR 方案就是它。

**版本是 Nuxt 3，不是 Nuxt 4。** `architecture.md:779` 与本文档原先都写「Nuxt 4」，而 `frontend/package.json` 锁的是 `nuxt: 3.21.11`（精确版本，非 range）。以 `package.json` 为准：它是运行的那个。`architecture.md` 那处尚未改，因为技术栈表属于决策记录，**如果当初确实定的是 Nuxt 4，那么现在装的是错版本，该改的是代码不是文档** —— 这一条留给决策者定。

**4A 之后代价变了（2026-08-25）：** 前台的三个页面、七个 composable、四个组件全部按 **Nuxt 3 的根级目录约定**写成（`pages/`、`composables/`、`utils/` 直接在 `frontend/` 下）。Nuxt 4 默认把这些移到 `app/` 子目录。所以升版本不再只是改一行依赖，还要搬整棵目录树。4A 期间实际踩过：`useEntryType.ts` 一度按 Nuxt 4 的习惯写进 `app/composables/`，不生效，移回根级才行。

**要改趁早。** 决定用 Nuxt 3 的话，改 `architecture.md` 技术栈表那一行即可，代码不动。

## 5. 目录结构现状

```
backend/
  cmd/
    server/main.go        启动：加载配置、装配依赖、监听信号
    cli/main.go           命令分派：config:check / user:create / session:prune
    cli/user.go           两个命令的实现 + 连接池装配
    cli/prompt.go         不回显读密码；拒绝 argv 传密码
  internal/
    apperr/               单一应用错误类型
    httpx/                响应信封（data / error 二者其一）+ request id
    config/               环境变量配置，零外部依赖
    postgres/             连接池；sqlcgen/ 为生成代码
    middleware/           通用 HTTP 关注点：RequestID / Logger / Recovery / CORS
      ratelimit.go        固定窗口计数器，仅挂在登录路由前
    health/               存活与就绪探针
    auth/                 认证领域，不依赖 Gin
      model.go            领域类型与 sentinel error
      password.go         Argon2id
      token.go            token 生成与 SHA-256
      repository.go       包装 sqlcgen，翻译驱动错误
      service.go          Login / Authenticate / Logout / CreateUser
      authtest/store.go   内存 Store，供两个测试包共用
    authhttp/             认证的 HTTP 适配层
      handler.go          三个端点
      middleware.go       鉴权中间件 + context 存取
      cookie.go           Cookie 属性集中一处
      dto.go              请求与响应形状
    entry/                内容领域，不依赖 Gin
      model.go            领域类型、sentinel error、slug 校验、CountWords
      repository.go       包装 sqlcgen，翻译约束违反（23505 / 23514 / 23503）
      service.go          12 个方法：建 / 读 / 改 / 软删 / 三个状态迁移 / 两条 Admin 读
      entrytest/store.go  内存 Store，供两个测试包共用
    entryhttp/            内容的 HTTP 适配层
      handler.go          10 个端点 + 领域错误到状态码的唯一映射
      dto.go              四种响应形状：前台列表 / 前台详情 / 作者详情 / Admin 列表
    taxonomy/             分类领域，不依赖 Gin。为「categories 今天、tags 以后」写的
      model.go            领域类型、sentinel error、name 与 slug 校验
      repository.go       包装 sqlcgen，翻译唯一索引违反
      service.go          8 个方法：建 / 两种读 / 两种列表 / 改 / 删 / ResolveSlug
      taxonomytest/store.go  内存 Store，供两个测试包共用
    taxonomyhttp/         分类的 HTTP 适配层
      handler.go          6 个端点
      dto.go              两种响应形状：公开（带计数）/ Admin（带时间戳）
    dbtest/               集成测试的库接入：前缀命名 + 按前缀清理
    router/               全部 URL 的唯一登记处
  migrations/             000001 扩展与触发器，000002 users + sessions，000003 entries，
                          000004 categories + entries.category_id
  sql/queries/            sqlc 输入：auth.sql / entry.sql / category.sql

admin/                    Vue 3 + TS + Vite（3A）
  src/
    api/
      client.ts           唯一的 HTTP 出口：信封拆解、错误归一、credentials
      errors.ts           ApiClientError + code 到中文文案的映射
      auth.ts             login / logout / fetchMe
      index.ts            对外的桶文件
    types/api.ts          按 api.md 抄的类型；枚举用 union
    stores/auth.ts        Pinia：user / isInitializing / initializationError
    router/index.ts       路由表 + beforeEach 守卫
    layouts/AdminLayout.vue   顶栏 + 侧栏 + 内容区
    views/                Login.vue / Dashboard.vue / NotFound.vue
    style.css             设计变量与元素默认值
  .env.example            VITE_API_BASE_URL
  vite.config.ts          端口固定 5173

frontend/                 Nuxt 3.21.11 + TS（4A 完成，见第 13 节）
  assets/css/
    tokens.css            设计值的唯一来源，不含组件样式
    main.css              reset 与元素默认值
    prose.css             .prose 下的全部样式（全局引入，见 13.1）
  pages/
    index.vue             列表页
    [slug].vue            详情页（根级 URL）
    categories/[slug].vue 分类页
  components/
    EntryTimeline.vue     按年份分组的时间线
    EntryRow.vue          单条列表项，版式随 type 变
    ThePager.vue          分页（链接，不是按钮）
    ThemeToggle.vue       主题切换
  layouts/default.vue     顶栏 + 内容区 + 页脚
  error.vue               错误页
  app.vue                 NuxtLayout + NuxtPage
  plugins/auth.server.ts  SSR 期间带 cookie 探一次 /me
  stores/auth.ts          Pinia：user / loading / initialized
  composables/
    useApi.ts             唯一的 HTTP 出口，拼 /api/v1 前缀
    useEntriesApi.ts      文章读取
    useCategoriesApi.ts   分类读取
    useSiteCategories.ts  分类列表的共享 useAsyncData
    useEntryType.ts       六种 type 到版式的查表
    useTheme.ts           主题读写（cookie）
  utils/
    api-error.ts          错误归一
    markdown.ts           markdown-it 实例与渲染
    date.ts               日期格式化，时区钉死 Asia/Shanghai
  types/                  api / auth / entry / category
  nuxt.config.ts          含字体反代的 routeRules，见 13.3
  .env / .env.example     NUXT_PUBLIC_API_BASE=http://localhost:8080
```

`internal/entry` 与 `internal/entryhttp` 的拆分理由与 auth 那对相同，见下面的偏离说明。三个适配层之间**没有依赖**：`entryhttp` 自己声明 `AuthorResolver` 与 `CategoryResolver` 两个函数类型，由 router 分别接到 `authhttp.Authenticated` 和 `taxonomy.Service.ResolveSlug`，所以谁都不用 import 谁。

`taxonomy` 没有 Clock，`entry` 有。这不是遗漏：分类的 `created_at` 来自列默认值、`updated_at` 来自触发器，这个包里没有任何东西需要盖时间戳。文章的 `published_at` 才需要。

`internal/media` 与 `internal/storage` **尚未创建**，连空目录都没有。

`admin/src/components/` 是空目录：3A 没有任何一处需要复用的片段，为「架构完整」先摆几个组件进去只会造出还没有第二个调用点的抽象。

### 测试现状

```
make test               197 个测试函数 / 372 处子测试，不需要数据库
make test-db-create     一次性：建 alive_test 并迁移
make test-integration   239 个函数 / 443 处子测试，-count=1 禁用缓存
```

共 239 个测试函数，其中 **42 个需要真库**（`internal/auth` 9、`internal/entry` 14、`internal/taxonomy` 13、`internal/postgres/sqlcgen` 6）。`TEST_DATABASE_URL` 未设时这 42 个自己 skip，`make test` 的 197 是剩下的部分。

**`make test` 全绿不等于存储层被验证过。** 需要真库的测试在 `TEST_DATABASE_URL` 未设时会自己 skip，必须用 `make test-integration` 才真的跑。这一点在上一个会话踩过：`make test-integration` 早先没有 `-count=1`，于是 `internal/auth` 命中缓存，"跑了集成测试"实际什么都没跑。

### 测试库隔离：为什么不能用 TRUNCATE

`internal/dbtest` 存在的原因是原先那套写法是错的，而且只有在第二张表出现后才暴露。

原先每个测试包在自己的测试前后 `TRUNCATE users CASCADE`。`go test ./...` **并行跑不同的包**，于是一个包的 truncate 会删掉另一个包正在用的行。只有 users 和 sessions 时窗口窄，很少中招；`entries` 一来，「建作者 → 插好几条 entry → 读回来」这个序列的窗口足够宽，作者会在测试中途被删掉，插入随即撞 `entries_author_id_fkey`。**实测 8 次跑挂 5 次。**

修法不是把包串行化，而是让每个测试不再假设自己独占整张表：行名带 `t<pid>x` 前缀，清理只删自己那些。**修后 10 次连跑 0 失败。**

`CleanupUsers` 先删 entries 再删 users，顺序不能反：`sessions` 从 users 级联，但 `entries.author_id` 是 `ON DELETE RESTRICT`——这是故意的，删账号不该把写过的内容悄悄带走。

`dbtest` 另外有一道硬闸：库名不以 `_test` 结尾就 `Fatal`，不是 skip。skip 会让配错的 DSN 伪装成一次绿色通过，而那正是这道检查要防的事。

### 与 `stage-auth-plan.md` 的一处偏离

计划文档把 `handler.go` / `middleware.go` / `cookie.go` 放在 `internal/auth` 内。实际放在了独立的 `internal/authhttp`。

原因：Go 的依赖是包级的，不是文件级的。若 `handler.go` 在 `internal/auth` 里 import Gin，则整个 `internal/auth` 包依赖 Gin，`cmd/cli` 只想用 `user:create` 也会链接进整个 Web 框架，而「auth 核心不依赖 Gin」就退化成只能靠 code review 维持的约定。

拆包后这个边界由编译器保证，且可验证：

```
go list -deps ./cmd/cli        | grep -c gin-gonic   →  0
go list -deps ./internal/auth  | grep -c gin-gonic   →  0
go list -deps ./internal/entry | grep -c gin-gonic   →  0
```

`internal/entry` / `internal/entryhttp` 沿用同一拆法，理由相同。`internal/health` 仍把 handler 放在包内，因为 health 没有非 HTTP 的使用者；auth 和 entry 有。

## 6. 关键决策速查

| 决策 | 取值 | 一句话理由 |
|---|---|---|
| 内容模型 | 统一 `Entry` + `type` 判别 + JSONB `meta` | 每种内容一张表会让「按时间列出全部」变成 N 路 UNION |
| 数据库 | PostgreSQL 18.6，启用 `citext` | 用户名大小写不敏感由列类型保证，不靠代码 lower() |
| 认证 | 服务端 session，非 JWT | 登出要能立即失效；JWT 做不到这点而不引入黑名单 |
| Token | 32 字节 crypto/rand，库存 SHA-256 | 库里存摘要，泄露后无法重放为 cookie |
| 密码 | Argon2id，PHC 字符串编码 | bcrypt 静默截断 72 字节且无内存成本 |
| Cookie | `HttpOnly` `SameSite=Strict` `Path=/api/v1` | Strict 是同域 `/admin` 决策换来的，子域方案只能用 Lax |
| 过期 | 7 天滑动，过半才续期 | 每请求续期会把每次读变成一次写 |
| 限流 | 进程内固定窗口，5 次/分钟/IP，仅登录 | Redis 会让限流器故障时连带拖掉登录 |
| 部署 | admin 在 `/admin`，与 API 同源 | 同源则生产环境无需 CORS，且 Strict 可用 |
| 对象存储 | OSS | |
| 错误 | sentinel error + `errors.Is`，驱动错误止于 repository | |
| 测试库 | 独立 `alive_test`，库名不以 `_test` 结尾则 Fatal | 集成测试会删行 |
| 测试隔离 | 行名带进程前缀，只清自己的行，**不用 TRUNCATE** | `go test ./...` 并行跑包，truncate 会删掉别的包正在用的行 |
| 响应信封 | 顶层只有 `data` 或 `error`，分页放 `meta` | `architecture.md:473` 写的顶层 `pagination` 是旧稿，实际以 `httpx` 为准 |
| 公开可见性 | 三个条件写在 SQL 里，调用方绕不过去 | 靠每个调用点记得加 `WHERE` 的话，漏一处就有一篇你以为删掉的日志出现在前台 |

## 7. Stage 2 已对齐的四项决策（均已落地）

已记录在 topic `topic_5669d7a3-bd05-4eb2-8bfd-dbad9711ca13`（revision 19），并含统一语言六个术语：`type`、`slug`、`status`、`visibility`、`happened_at`、`published_at`。

| 项 | 结论 | 理由 |
|---|---|---|
| `category_id` | **本次不建**。等做分类时用 000004 同时建 `categories` 表与该列 | 一个指向不存在表的列无法建外键，引用完整性在这期间毫无保障，还容易被误当作已可用 |
| `slug` | **客户端必传**，后端不生成。格式限小写字母/数字/连字符，重复返回 409 | 中文标题自动转 slug 只有两条路：百分号编码得到不可读 URL，或引入拼音库把展示问题变成后端依赖 |
| 字段范围 | **按 `architecture.md` 2.2 节全建**（除 `category_id`）。实际落地为 5 个部分索引 + 1 个 GIN + 1 个部分唯一索引；第六个部分索引 `idx_entries_category` 依赖 `category_id`，同样推迟到 000004 | 一次建对后续只动代码不动 schema；加列便宜，改约束和补索引不便宜 |
| `type` | **六值一次定齐**：`journal` `book` `movie` `music` `travel` `photo`，用 CHECK 约束限定。本次只实现 `journal` 的 meta 结构 | 一个 `joural` 错别字会让那条记录从所有按类型筛选的查询里消失，且不报任何错 |

新增第七个 `type` 需要一个 migration。这个成本是有意保留的：它强制新类型经过一次显式决策，而不是随手写一个字符串。

### 落地时发现的两处文档冲突

两处都按较新的显式决策执行。**两处的 `architecture.md` 已于 2026-08-25 改完，此处保留是为了记住分歧曾经存在过：**

1. `architecture.md:258` 曾说 `type` 不加 CHECK 约束，与上表第四项相反。按上表执行，加了 CHECK；文档已改为记录这个决策和推翻的理由。
2. `architecture.md:481` 说 `visibility=unlisted` 可凭链接通过详情接口取到，与当时「公开端点只返回 `visibility='public'`」的实现相反。2E 按文档执行了：加了一条独立语句 `GetLinkEntryBySlug`，而不是在现有公开查询上加参数——那等于给公开路径开一个可被传参绕过的口子。见第 9 节。

另有一处名词需要澄清：本文档旧版把开发库账号记为 `owner`，而 Makefile 里的 DSN 用户是 `alive`。两者不是一回事：`alive` 是 PostgreSQL 角色，`owner` 是应用内的账号。

## 8. 2D 已对齐的五项决策（均已落地，2026-08-25）

记录在 topic `topic_b62437fd-bfb1-44b6-ab3e-7da2fafb0a8e`，统一语言四个术语。**本阶段没有新 migration**：`deleted_at`、`status`、`published_at`、`entries_published_at_check`、`idx_entries_admin`、部分唯一索引 `uk_entries_slug` 在 000003 里已经齐了。

| 项 | 结论 | 理由 |
|---|---|---|
| 改的语义 | **只做 `PATCH` 局部更新**，不提供 `PUT` | `PUT` 要求客户端回传整篇。两个标签页同时开一篇文章时，后保存的那个会覆盖前一个改过却没提及的字段，而且悄无声息 |
| Admin 前缀 | **独立 `/api/v1/admin/*`**，SQL 也是独立语句，不是公开查询加布尔开关 | 一个切换可见性过滤的参数，离「传错一个实参就把全部草稿发出去」只有一步。路径本身说明了这批语句能看到什么 |
| 发布 | **独立端点 `publish` / `unpublish`**，不做 `PATCH` 里的 status 字段 | `published_at` 写一次就不再移动，这条规则只应有一个落点。`PATCH` 收到 status 直接 400 |
| 删 | **只做 `DELETE` 软删**，没有硬删，也没有恢复端点 | 内容不可再生。行留着但每个读都过滤 `deleted_at`，所以它不可达而非不存在 |
| 归档 | **加第三个端点 `archive`** | 不加的话 `archived` 是数据库接受、却没有任何路径能写进去的状态：`PATCH` 拒收 status，publish / unpublish 只写 published 和 draft，只有 Create 能设。而「没写完」和「写完了又撤下」是两件事，Admin 列表要区别对待——草稿是待办队列，归档不是 |

### 局部更新为什么不能用 COALESCE

`summary`、`cover_url`、`happened_at` 三列可空。用 `COALESCE(:field, field)` 时，「清空这个字段」和「别动这个字段」都是一个 NULL 实参，两者变成同一条语句，其中一个必然无法表达。

所以 `UpdateEntry` 每个字段配一个布尔标志：

```sql
summary = CASE WHEN sqlc.arg(set_summary)::boolean
               THEN sqlc.narg(summary)::text ELSE summary END
```

清空是 `set_summary = true` 且 `summary = NULL`，与 `set_summary = false` 截然不同。Go 侧对应 `UpdateInput` 的全指针字段：`nil` 是未提交，指向零值是提交了一个零值。

一处需要注意的取舍：JSON 的 `null` 和「不写这个 key」在 `encoding/json` 里都让指针保持 `nil`，两者不可区分。所以清空走空值（`""`，`happened_at` 走零值时间），不走 `null`。要区分它们得解码进 `map[string]json.RawMessage`，那就丢掉了结构体的类型检查。

### `published_at` 为什么必须一条语句写完

`entries_published_at_check` 拒绝 `status='published'` 且 `published_at IS NULL` 的行。先改 status 再补时间戳的两步写法，第一步就会被约束打回。所以：

```sql
SET status = 'published',
    published_at = COALESCE(published_at, sqlc.arg(published_at))
```

`COALESCE` 就是那条「首次发布时间不再移动」的规则本身：一篇发布过、撤下、再发布的文章保留原始日期，三年前的旧文修个错字不会跳到时间线顶部。unpublish 与 archive 都不碰这一列。

### 软删会释放 slug

`uk_entries_slug` 是部分唯一索引（`WHERE deleted_at IS NULL`），所以软删后那个 slug 可以被新文章占用。这是有意的，也有集成测试盯着：`TestRepositorySoftDeleteReleasesTheSlug`。

同一个索引也是「更新时改 slug 要排除自己」的原因——`EntrySlugExistsExcluding` 带 `AND id <> :excluded_id`，否则编辑器每次连着原 slug 一起提交都会撞 409。

### 七个新端点的实机验证（2026-08-25）

同 2C 的做法：在 `alive_test` 上起真实服务（8099），用后即清，开发库零残留（已核对 `patch-probe%` 为 0 行）。

- `PATCH` 只送 `title`，另外四个字段（`summary`、`cover_url`、`content_md`、`word_count`）与 `status` 全部原样——这是 CASE 表达式在真库上的证据
- 送 `""` 清空了 `summary` 与 `cover_url`；送 `null` **没有**清空，与上面「null 读作未提交」的说明一致
- publish → unpublish → publish，三次返回的 `published_at` 逐字符相同（中间隔了 2 秒），撤下时该字段也没被清
- `archive` 后前台 404、Admin 按 id 仍 200、`published_at` 仍在
- `status` 筛选：draft 0 条、published 0 条、archived 1 条；`status=stauts` 得 400 而不是空列表
- `DELETE` 第一次 204 且响应体 0 字节，第二次 404；删后前台、Admin 详情、Admin 列表三条路径同时取不到
- **软删后同一个 slug 能被新文章占用，返回 201**，即部分唯一索引确实释放了它
- 直接查库确认该行仍在：`status=archived`、`deleted_at` 非空、`published_at` 非空。软删只加了一列时间戳，没动别的
- 匿名打 `/admin/entries` 得 401；`PATCH /entries/abc` 与 `/entries/0` 得 400（不是 404），`publish /entries/999999999` 得 404

### 这次新增的测试

- `internal/entry` 领域层 8 个函数，覆盖 Set 标志的取舍、空输入拒绝、只校验已提交字段、slug 自身排除、软删的四个后果、发布写一次、Admin 列表排序与筛选、以及 `id <= 0` 在六个方法上都不发查询
- `internal/entry` 集成层 9 个函数，这些只有真库能验证：CASE 表达式确实不动未标记的列、三个可空列能被清空、`COALESCE` 不移动日期、部分唯一索引释放 slug、Admin 语句看得见全部状态
- `internal/entryhttp` 13 个函数，含 `archive` 既不是草稿也不是删除、Admin 读取与公开读取在同一批种子数据上结论相反
- `internal/router` 把受保护路由从 1 条扩到 8 条，逐条断言 401。这是唯一能证明新端点真的挂在 `RequireAuth` 后面的地方，其中两条 Admin 读取最关键：它们是全服务唯一能返回草稿的路由

## 9. 2E 已对齐的三项决策（均已落地，2026-08-25）

本阶段有一个新 migration：`000004`，加 `categories` 表、`entries.category_id`、以及此前推迟的 `idx_entries_category`。因此当时是 **6 个部分索引 + 1 个 GIN + 1 个部分唯一索引**，与 architecture.md 一致，3.1 里那条「差一个索引」的记录已经作废。（4A 的 000005 又加了表达式索引 `idx_entries_public_timeline`。`entries` 上现为 **6 个非唯一部分索引 + 1 个 GIN + 1 个部分唯一索引**，已用 `pg_indexes` 核对，见 13.5。）

| 项 | 结论 | 理由 |
|---|---|---|
| unlisted | **沿用文档：slug 直接可达**。加一条独立语句 `GetLinkEntryBySlug`，接受 `visibility IN ('public','unlisted')`；列表与计数仍然只认 `public` | 备选方案是 `share_token`，即凭一个不可猜的令牌访问。但那是访问控制，成本是一列、一次生成、一次轮换；而 unlisted 要的只是「不广播」。两者不该混在一个概念里 |
| categories 范围 | **完整落地**：表 + CRUD + 挂到 Entry | 只加表不加 CRUD 的话，分类只能用 psql 建，等于把一个功能做成一半 |
| 删分类 | **直接删，文章变未分类**。`ON DELETE SET NULL` | 备选是 RESTRICT（先搬空才能删）或软删。RESTRICT 会让「删掉一个用错的分类」变成一次批量操作；软删则要求每个读都过滤，为一张导航表付这个代价不值得 |

第三项的代价当时就指出并接受了：**这个操作没有痕迹**。删完之后没有任何地方记得那些内容原来属于哪个分类，一次 204 可能静默改掉很多行。所以 `taxonomy.Service.Delete` 为每次删除写一条 INFO 日志——那是唯一的证据。

### unlisted 为什么是两个方法而不是一个布尔参数

`entry.Entry` 上是 `IsPubliclyReadable()` 和 `IsLinkReadable()` 两个方法，SQL 里是 `GetPublicEntryBySlug` 和 `GetLinkEntryBySlug` 两条语句。一一对应是有意的：一个 `includeUnlisted bool` 参数，离「某个调用点传错实参就把 unlisted 全列出来」只有一步，而两条语句里没有任何地方能被传参绕过。

同一个理由，`ListPublic` 的 `categoryID` 过滤只会收窄可见范围：分类筛选和可见性过滤是 AND，不是 OR。

### 读和写为什么在分类上不对称

读路径 `LEFT JOIN categories`，所以 `CategoryName` / `CategorySlug` 有值。写路径不能 join：Postgres 的 `RETURNING` 只看见被写的那一行。

于是响应形状分开：
- `entrySummary.category` 是嵌套对象，读到就有，未分类是 `null`
- `ownerEntryDetail` 同时带 `category_id`（写和读都有值）和 `category`（写后是 `null`）

不选「写完再读一次」是因为那是每次保存多一个查询；不选「写响应里发 `{"id":3,"name":"","slug":""}`」是因为 `name` 是 NOT NULL 且有长度 CHECK，空名字的分类不存在，发出去等于对外声明一个不可能存在的对象。编辑器确认保存成功时读 `category_id`，它两种情况下都有值。

### `?category=` 为什么解析 slug 而不是直接进 SQL

`?category=nope`（链接打错）和 `?category=空分类` 是两回事。前者必须 404：客户端分不清就会把打错的链接显示成「这个分类下暂无内容」。

解析发生在 `router.categoryFromSlug` 里，而不是 `entryhttp` 里。`entryhttp` 不 import `taxonomy`，所以它认不出 `taxonomy.ErrCategoryNotFound`；能认出的那一侧负责把它翻成 404，其余错误原样落成 500。`entryhttp` 声明的 `CategoryResolver` 类型就是这个边界。

`?category=`（空串）当作没传。前端从表单状态拼查询串时，分类选「全部」发出的就是这个。

### 六个新端点与两项改动的实机验证（2026-08-25）

同 2C / 2D 的做法：在 `alive_test` 上起真实服务（8099），用后即清（已核对 `catprobe%` 三张表均为 0 行，开发库零写入）。

分类本身：

- 建两个分类返回 201；重复 slug 409 且 `fields.slug`；`"Not A Slug"` 400 且 `fields.slug`
- 匿名写 401、匿名读公开列表 200、匿名读 `/admin/categories` 401
- 公开列表字段 `[description, entry_count, id, name, slug, sort_order]`，Admin 列表字段 `[created_at, description, id, name, slug, sort_order, updated_at]` —— 两边一个带计数、一个带时间戳，互不重叠
- 公开列表**不含 `meta`**，即确实没有分页
- `PATCH` 六种情况逐一确认：只送 `name` 时 description 与 sort_order 原样；`description:null` **没有**清空；`description:""` 清空了；`sort_order:0` 写进去了；空 body 400；改成另一个分类的 slug 409，改成自己的 slug 200

挂到 Entry 之后：

- `?category=catprobe-travel` 下该分类共 4 篇（2 篇公开、1 篇 unlisted、1 篇草稿），列表返回 2 篇且 `meta.total=2` —— 分类筛选没有放宽可见性
- `?category=nope-nope` 得 **404**，不是空列表；`?category=`（空串）得完整列表
- `catprobe-unlisted-a` 凭 slug 能取到，带正文、带嵌套 category；同一篇在公开列表和筛选后的列表里都是 0 次出现；草稿凭 slug 是 404
- 未分类文章的 `category` 是 `null`，不是空对象
- `category_id` 指向不存在的分类：Create 与 PATCH 都得 400 且 `fields.category_id`
- `PATCH {"category_id":0}` 之后 `category_id=0`、`category=null`

删分类（决策三的实机验证）：

- 第一次 204、第二次 404
- 删之前 entry 1308 的 `category_id=69`；删之后 `category_id=0`、`category=null`、**`status` 仍是 published**，前台仍然 200 —— 文章活下来了，只是没有分类
- 用被删的 slug 再筛选得 404
- 服务日志里有且仅有那一行 `category deleted, its entries are now uncategorised category_id=69`。这是这个操作留下的唯一痕迹

### 这次新增的测试

- `internal/taxonomy` 领域层 19 个函数：Set 标志的取舍、`""` 清空与 `null` 不清空、零值 `sort_order` 是真实位置、slug 自身排除、pre-check 漏掉的竞态由约束兜住、`id <= 0` 不发查询、以及 9 个存储故障不被伪装成领域错误
- `internal/taxonomy` 集成层 12 个函数，其中三件只有真库能证明：唯一索引翻译成 `ErrSlugTaken`、计数的 `LEFT JOIN` 只数读者能看到的内容（草稿 / 归档 / private / unlisted 各一条种子，计数是 1）、以及 **`ON DELETE SET NULL` 确实让文章活下来并变成未分类**——最后这条是决策三的唯一验证点，RESTRICT 或 CASCADE 写错都会在这里失败
- `internal/taxonomyhttp` 20 个函数：公开列表带计数不带时间戳、Admin 列表反之、空列表序列化成 `[]` 而不是 `null`、不带分页 meta、非数字 id 是 400 而不是 404、nil guard 直接 panic
- `internal/entryhttp` 新增 12 个函数：`?category=` 同时过滤页和总数、未知 slug 404、空串等同未传、筛选后仍然藏住未发布内容、四种响应形状里的 `category` 对象、写响应的 `category_id` 有值而 `category` 是 `null`、`{"category_id":0}` 变未分类而 `null` 不动
- **`internal/entryhttp` 原有的 `visibilityCase` 从一个 `visible` 标志拆成 `inList` + `byLink`**。unlisted 是唯一两者不同的一行，一个标志表达不了
- `internal/router` 三件事：分类路由的 5 条写 + 1 条公开读、`TestNewRequiresTaxonomyService`、以及 `TestTheCategoryFilterIsWired`——后者是唯一能发现「resolver 没接上」的地方，两个适配器各自的测试都用 fake，都会通过。已验证：把 router 里的 resolver 改成 `nil`，这个测试失败（500 而非 404）

### dbtest 新增 `CleanupCategories`

按 slug 前缀删，和其余清理一样。不写 entries 语句，也不需要：`entries.category_id` 是 `ON DELETE SET NULL`，删分类不碰内容。这和 `author_id` 的 RESTRICT 正好相反——`CleanupUsers` 必须先删 entries 才能删 users。

## 10. 3A：`admin/` 工程初始化与认证闭环（2026-08-25）

只做认证与骨架。文章 CRUD、Markdown 编辑器、自动保存、分类管理、tags、media、archives、site 均**不在**本次范围。后端一行未改（`git diff -- backend/` 为空）。

### 技术栈

`npm create vite@latest admin -- --template vue-ts` 拉到的当前稳定版：

| 包 | 版本 |
|---|---|
| Vue | 3.5.41 |
| TypeScript | 6.0.2 |
| Vite | 8.2.2 |
| vue-router | 5.2.0 |
| Pinia | 4.0.3 |
| vue-tsc | 3.3.11 |

手动装的依赖只有 `vue-router` 与 `pinia`。没有 UI 库；没有 axios —— 原生 `fetch` 够用，且少一层要理解的东西。

strict 来自 `@vue/tsconfig` 的 `strict: true`，脚手架另给的 `noUnusedLocals` / `noUnusedParameters` / `noFallthroughCasesInSwitch` 都保留。全项目零 `any`。

### API Client 的三个决定

**一、信封在 client 里拆完，页面拿到的是 `User` 而不是 `{ data: User }`。**

底层是 `requestEnvelope`（返回整个信封），上面两个出口：`request` 取 `data`，`requestPaginated` 保留 `meta`。分页端点少，但把 `meta` 丢在 client 里会逼将来的列表页自己发第二次请求。

`requestPaginated` 在缺 `meta` 时抛 `INTERNAL` 而不是伪造一个默认值。`GET /categories` 就是不带 `meta` 的，用错函数应当当场炸而不是静默给出「一页，总数等于本页条数」——后者会让分页控件显示正确却永远翻不到第二页。

**二、每个请求自动带 `credentials: 'include'`，不给调用方选择。**

会话是 HttpOnly Cookie，漏一个就是「登录看起来成功了但后续全 401」。这类错误没有编译期信号，所以不能靠记性。

代码里没有任何 token 处理，也没有 `localStorage` / `sessionStorage` / `document.cookie` / `Bearer`。全仓库唯一出现 `localStorage` 字样的地方是 `client.ts` 里解释为什么不用它的注释。

**三、错误归一成 `ApiClientError`，文案不取后端 `message`。**

带 `code` / `status` / `fields` / `requestId` / `retryAfter`。网络层失败（离线、DNS、CORS 被拒、abort）没有响应体，给一个本地伪码 `NETWORK`，于是调用方只有一种东西要 catch，也不必判断「这是不是一个真的 HTTP 错误」。

文案映射在 `errors.ts` 的 `toUserMessage`。**后端 `message` 一个字都不进 UI**：它是写给读日志的人的，而 500 时它正是驱动层字符串最可能漏进界面的地方。`INVALID_INPUT` 那一支还要看 `status`——405 也是这个 code，那是调用方写错了方法，不是用户输入有问题。

### 401 为什么用注册式回调

`client.ts` 暴露 `setUnauthorizedHandler`，`main.ts` 在启动时把它接到 store 和 router：

```
main.ts  →  setUnauthorizedHandler(清 store + 跳 /login)
```

直接在 client 里 import store 和 router 会形成 `store → api → store` 的循环依赖。反过来让每个页面自己处理 401，则等于把同一段逻辑抄 N 遍，且漏掉任何一处都表现为「界面还在但什么都点不动」。

**两个端点显式跳过这个回调**（`skipUnauthorizedHandler: true`）：

- `login` —— 它的 401 是 `INVALID_CREDENTIALS`，意思是「密码错了」。跑会话过期逻辑会清空状态并跳到用户已经在看的那一页，把他需要看到的报错顶掉。
- `/me` —— 它的 401 是「没登录」，而调用方就是 auth store，它自己会解释。让回调也跑等于两处响应同一个 401，且在启动阶段会在路由决定任何事之前就跳转。

这正是 `api.md` 1.2 强调 `INVALID_CREDENTIALS` 与 `UNAUTHORIZED` 分开的实际用途：两个都是 401，但一个留在原地报错，一个跳走。

### Auth Store：三态而非两态

`user` / `isInitializing` / `initializationError` / `isAuthenticated` / `displayName`，动作 `login` / `logout` / `fetchMe` / `initialize` / `clearSession`。

**`fetchMe` 把 401 当正常答案**（返回 `false` 并清 user），其他错误照抛。这条边界是有意的：「服务器坏了」不能被记成「你没登录」，否则后端一挂，界面就显示一个登录页，而登录同样不可能成功。

**`initialize` 永不 reject。** 失败落进 `initializationError`，所以守卫永远能完成路由。它还去重——并发调用共享同一个 promise，否则守卫和启动流程会各发一次 `/me` 并抢着写状态。

**`initializationError` 与「未登录」是分开的两件事**，登录页因此能说「无法连接到服务器」，而不是让人怀疑自己记错了密码。

`displayName` 在 `display_name` 为空时回退到 `username`。数据库里该列可空且序列化成 `""`，所以空串和缺失要同样对待。

`role` **没有**做成 union type：`migrations/000002` 里它是 `VARCHAR(16)` 带默认值 `'owner'`，**没有 CHECK 约束**，列注释也明说「记录但从不读取」。写成联合类型等于替 schema 承诺了它没有承诺的事。`status` / `visibility` / `type` 有 CHECK 约束，都做了 union。

### Router Guard：`await` 而不是读标志

守卫里是 `await auth.initialize()`，不是读 `isInitializing` 然后期待它已经是 `false`。

这一条是整个 3A 最容易写错的地方。错误写法的表现很具体：

```
刷新页面 → 守卫立刻跑 → /me 还没回来 → isAuthenticated 是 false → 跳 /login
```

已登录的人每次刷新都被踹回登录页，而登录页因为 `guestOnly` 又把他送回来，看起来像闪烁。`initialize` 自带去重，所以首次之后每次导航拿到的都是已 resolve 的 promise，等待成本为零。

`main.ts` 里也 `await` 了一次才 `mount`。这一次不是为了正确性（守卫已经保证了），而是为了避免挂载后先画一帧未登录外壳再跳转。

回跳只接受以 `/` 开头的路径：`redirect` 来自 URL，跟着它跳绝对地址会让这个表单变成一个开放重定向。

路由表里 `/entries`、`/entries/new`、`/entries/:id`、`/categories` **没有预留占位路由**。侧栏把 Entries 与 Categories 渲染成 `aria-disabled` 的文字而不是链接：指向一个不存在页面的链接比没有链接更糟。

### 实机验证（浏览器，Playwright 驱动真实 Chromium）

后端 8080 + Vite 5173，全部走真实 Cookie：

| 场景 | 结果 |
|---|---|
| 未登录访问 `/dashboard` | → `/login?redirect=/dashboard` |
| 正确登录 | `Set-Cookie` 建立，回跳到 `/dashboard` |
| **刷新页面** | 停在 `/dashboard`，`/me` 恢复状态，**不误跳** |
| 已登录访问 `/login` | → `/dashboard` |
| 访问 `/` | → `/dashboard` |
| 退出 | 204 + `Max-Age=0`，回 `/login`，且服务端会话真的失效 |
| 错误密码 | `INVALID_CREDENTIALS` → 「用户名或密码错误」 |
| **会话中途被吊销**（直接 `delete from sessions`） | 401 handler 触发，跳登录并保留 redirect |
| 后端进程停掉 | 初始化提示 + 「无法连接到服务器」，应用不卡死仍能路由 |
| **连点 3 次登录按钮** | 只发出 **1** 个请求 |
| 连续错误密码触发 429 | 「尝试次数过多，请稍后重试。」 |

连点那条是用 Playwright 同时发三次 `click({ force: true })` 做的——服务端密码哈希约 200ms，三次点击都落在同一个在途请求内。计数来自监听 `page.on('request')`，不是看界面。

`isSubmitting` 的判断放在 `handleSubmit` 函数体里而不是只靠按钮的 `disabled`：文本框里按 Enter 也会触发提交，`disabled` 拦不住那条路径。

### 3A 期间发现的三处文档与实现不一致

**一、端口容易被误读。** `api.md` 开头说「对着 `alive_test` + 8099 端口核对过」——这句本身是准确的，它陈述的是验证环境，不是默认值。但只读那一句很容易以为后端跑在 8099，而 `.env` 里是 `SERVER_PORT=8080`。`admin/.env.example` 按实际默认值配了 8080，环境事实表也补了这一行。`api.md` 第 11 行已补一句说明默认端口。

**二、`Retry-After` 浏览器读不到。** 详见 3.7。这一条是先从 CORS 配置推出来、再用浏览器实测确认的：curl 能看到头，页面上却只出现兜底文案。

**三、开发库缺 `categories`，`GET /entries` 返回 500。** 详见 3.1。这是本次唯一一个真正阻塞下一阶段的发现，也与 admin 无关——后端在开发库上本来就是这个状态。

### Dashboard 上没有数字

后端没有 dashboard 统计端点，所以 Dashboard 只有问候语和一个「最近编辑」占位块。

没有文章总数、阅读量、访问量、增长率。**编出来的数字和真实数字在界面上长得一模一样**，这正是它比一片空白更糟的原因：空白会促使人去把接口做出来，假数字会让人以为已经做完了。

### 清理

验证期间建过临时账号 `admintest` 和一个临时哈希工具 `backend/cmd/tmphash`，**都已删除**，库里只剩 `owner`。`git diff -- backend/` 为空。

事后发现建临时账号是多余的：`progress.md` 环境事实表里一直写着 `owner` 的开发密码，且当天验证仍然有效。**下次需要登录先读那张表。** 环境事实表现在明确写了这一句。

`npm run build` 通过：43 modules，JS 100.74 kB（gzip 39.76 kB）。中途 `vue-tsc` 报过一次 TS2352——`isRecord` 收窄后直接断言不合法，改成经 `unknown` 转换并注释了为什么不逐字段校验（API 契约就是那个保证，再校验一遍等于维护第二份 schema）。

## 11. 三端联合启动验证（2026-08-25）

第一次把后端、`admin/`、`frontend/` 同时跑起来。结论：**三端都能启动**，过程中暴露两个真问题，均已修。

### 实测结果

```
                    ┌──────────────────────┐
  :3000  Nuxt 前台 ─┤ CORS 白名单           │
                    │ 3000 / 5173（精确匹配）├─→ :8080 Go ─→ :5432 PostgreSQL 18.6
  :5173  admin 后台─┤                      │      schema v4
                    └──────────────────────┘
```

| 组件 | 构建 | 运行 | 证据 |
|---|---|---|---|
| backend | 通过 | :8080 | `/health/ready` 200，池 6/25 空闲，DB 延迟 376µs |
| `admin/` | 通过 167ms | :5173 | `vue-tsc` 类型检查过；43 modules |
| `frontend/` | 通过 | :3000 | `.output` 2.22 MB（gzip 548 kB） |

`owner` / `alive-dev-password-2026` 登录闭环：登录 200 且下发 `alive_session` cookie，带 cookie 请求 `/me` 200、`/api/v1/admin/entries` 200。错密码 401 `INVALID_CREDENTIALS`，空 body 400 `INVALID_INPUT`。5173 的 CORS 预检 204，带 `Allow-Credentials: true` 与精确 origin。

### 问题一：开发库缺 migration 000004

见 3.1。`/entries` 与 `/categories` 双双 500。已 apply，库到版本 4。

### 问题二：`frontend/` 的 API 端口写成 8099

`nuxt.config.ts` 的 `runtimeConfig.public.apiBase` 默认值、`.env.example`、`README.md` 两处都写 `http://localhost:8099`，而后端在 8080。SSR 阶段直接暴露：

```
WARN [auth] session check failed:
     [GET] "http://localhost:8099/api/v1/me": <no response> fetch failed
```

这不是「哪个端口更好」的问题：`backend/.env`、`backend/.env.example`、`admin/.env.example` 三处都是 8080，**8099 是唯一的离群值**，来源是 2C 那次用测试库验证的临时端口被当成了默认值抄进前端。四处已全部改为 8080。

`frontend/.env` 原本不存在，只有 `.env.example`，于是实际生效的是 `nuxt.config.ts` 里那个错的 fallback —— 两个问题叠在一起才让它一直没被发现。已创建 `frontend/.env`，并确认三个 `.env` 都在各自 `.gitignore` 内。

改完重启 Nuxt、不带任何环境变量覆盖，渲染出的 payload 里 `apiBase:"http://localhost:8080"`，session 探测报错归零。

### 两个已知噪音，未处理

Nuxt dev 启动时反复报 `Failed to resolve import "#app-manifest"` 的 pre-transform error。页面照常 200 渲染，SSR 与 Nitro 构建都正常，未深查。

裸 `go build` 在此网络下必挂（`proxy.golang.org` i/o timeout）。Makefile 已内置 `GOPROXY=https://goproxy.cn,direct`，**走 `make` 而不是直接 `go build`**；环境事实表早有这一条，这次又踩了一次。

### 未验证

`make test` 与 `make test-integration` 这次都没跑，测试是否全绿未知。`entries` 与 `categories` 均为 0 行，前台没有真实数据可展示。三端的浏览器实测只到「页面返回 200」，没有点过 UI。

## 12. 3B：`admin/` 内容管理（2026-08-25）

3.6 列的四步全部落地：migration（3.1 已解决）、分类管理、文章列表、Milkdown 编辑器。

### 12.1 新增的文件

```
admin/src/
├── api/
│   ├── categories.ts      读写路径不对称都收在这里
│   ├── entries.ts         列表 / 详情 / CRUD / 三个状态迁移
│   └── patch.ts           api.md 1.7 的唯一实现处
├── components/
│   ├── CategoryForm.vue   新建与编辑共用，不发请求
│   ├── EntryRow.vue       列表一行
│   └── MarkdownEditor.vue Milkdown 封装
└── views/
    ├── Categories.vue     列表 + 行内删除确认
    ├── Entries.vue        状态页签 + 分页
    └── EntryEditor.vue    新建与编辑共用
```

`types/api.ts` 补齐了 Category 与 Entry 两组类型。路由新增 `entries`、`entry-new`、`entry-edit`、`categories`，`AdminLayout` 里对应的导航项解锁。

### 12.2 三个值得记的实现决定

**`patch.ts` 是 1.7 规则的唯一住址。** 每个字段和加载时的原值逐项比对，只有真的变了才进 body。`happened_at` 清空发的是零时间戳 `0001-01-01T00:00:00Z`，不是 `null`。`isEmptyPatch` 在发请求前拦下空 patch —— 空 body 是 400，而「什么都没改」不该报错。视图层不碰这些规则。

**`EntryUpdateRequest` 里没有 `status` 字段。** PATCH 带 `status` 会 400，所以让类型系统直接堵住这条路，而不是靠注释提醒。发布走三个专用端点。

**`MarkdownEditor` 不是 `v-model`。** Milkdown 只在构造时读一次初始内容，之后文档归它自己管。每次变更再把 prop 灌回去，要么打断输入和撤销栈，要么被忽略。所以初始值读一次，变更单向往外走 `update` 事件。代价是调用方必须等内容加载完再挂载，`EntryEditor` 用 `v-if` 加 `:key="original?.id ?? 'new'"` 处理。

另有两处小的：文章列表按 `?status=` 走**服务端**筛选，客户端切分页数据只会漏报；新建保存成功后 `router.replace` 到编辑路由，否则第二次保存会用同一个 slug 再 create 一次，撞 409。

### 12.3 Milkdown 的实际成本

`@milkdown/kit` + `@milkdown/vue` + `@milkdown/theme-nord`，均锁 `7.22.1`。

编辑器被拆成懒加载 chunk：**459.93 kB / gzip 139.32 kB**，只在打开编辑页时下载，列表页不受影响。CSS 从 6.14 kB 涨到 22.56 kB —— 主题样式必须显式 import 三个文件（`prosemirror.css`、`gapcursor.css`、`theme-nord/style.css`），少了任何一个编辑器就是无样式的裸块。这一点在 `MarkdownEditor.vue` 里注释了。

### 12.4 写路径实测（对着运行中的后端）

| 验的是什么 | 结果 |
|---|---|
| `PATCH {"status":"published"}` | 400，`fields.status: use POST /entries/:id/publish or /unpublish` |
| 发布 → 撤回 → 隔 2 秒再发布 | `published_at` 微秒级完全一致，没有前移 |
| 写响应的 category 形状 | `category_id: 2` 但 `category: null` |
| admin 读响应的 category 形状 | `category_id: 2` 且 `category: {id, name, slug}` |
| `happened_at` 设置后用零时间戳清空 | 变回 `null` |
| `category_id: 0` | 变为未分类 |
| `category_id: 9999` | 400，`fields.category_id: no category has this id` |
| 创建时 slug 重复 | 409，`fields.slug: already taken` |
| PATCH slug 改成它自己 | 200，不会自我冲突 |
| 非法 `type` | 400，列出六个合法值 |

`admin/` 构建通过（930 modules，`✓ built in 414ms`），三个新模块在 dev 下都能 transform，`/entries`、`/entries/new`、`/entries/1` 三条路由都返回 200。

### 12.5 没验证的

**界面没有点过。** 环境里没有浏览器自动化，装一套的代价超过这次任务本身。所以「Milkdown 敲字能出富文本」「页签点了会切」「删除确认会拦住」这些都只是代码上应该成立，没有实证。

`make test` 与 `make test-integration` 这次仍未跑。

开发库里留了我造的验证数据，不是真实内容：分类 `旅行/travel`（id 2），文章 `first-post`（草稿）、`kyoto-spring`（已发布/公开/旅行）、`reading-notes`（已发布/不列出）、`old-review`（已归档）。要清掉说一声。

---

## 13. 4A：`frontend/` 前台页面与设计系统（2026-08-25）

三个页面加一套设计系统。排印是这个站的产品，所以 token 层不是「顺手抽的变量」，而是这一阶段的主要交付物。

### 13.1 新增文件

**样式层**

| 文件 | 作用 |
|---|---|
| `assets/css/tokens.css` | 设计值的唯一来源。字体、字阶、行高、间距、两套配色。**不含任何组件样式** |
| `assets/css/main.css` | reset 与元素默认值。只通过变量读值 |
| `assets/css/prose.css` | `.prose` 下的全部样式，即 Markdown 渲染结果 |

`prose.css` 是全局引入而非 scoped：scoped 样式加不到 `v-html` 的内容上，那段 HTML 不带组件的生成属性。

**逻辑层**

| 文件 | 作用 |
|---|---|
| `utils/markdown.ts` | markdown-it 实例与渲染函数 |
| `utils/date.ts` | 日期格式化，时区钉死 `Asia/Shanghai` |
| `composables/useEntryType.ts` | 六种类型 → 版式规则的查表 |
| `composables/useTheme.ts` | 主题读写 |
| `composables/useSiteCategories.ts` | 分类列表的共享 `useAsyncData` |

**视图层**

| 文件 | 作用 |
|---|---|
| `pages/index.vue` | 列表页（重写，原为十行占位） |
| `pages/[slug].vue` | 详情页 |
| `pages/categories/[slug].vue` | 分类页 |
| `components/EntryTimeline.vue` | 按年份分组的时间线 |
| `components/EntryRow.vue` | 单条列表项，版式随类型变 |
| `components/ThePager.vue` | 分页 |
| `components/ThemeToggle.vue` | 主题切换 |
| `layouts/default.vue` | 站点框架（重写） |
| `error.vue` | 错误页（重写） |

### 13.2 设计方向

**方向：墨与纸的编辑排印**（ink-on-paper editorial）。中文正楷做正文，一切层级靠字号、颜色、留白，不靠字重。

**设计签名：类型差异化排版**（type-differentiated layout）。六种 `type` 不是六个标签，是六种版式：

| type | 封面处理 | 摘要 |
|---|---|---|
| `journal` 日志 | 不显示 | 显示 |
| `book` 书 / `movie` 影 / `music` 乐 | 行内小图，2:3 竖版（书封、影海报） | 显示 |
| `travel` 行 | 领头大图，3:2 | 显示 |
| `photo` 影像 | 领头大图，3:2 | 不显示（图就是内容） |

规则集中在 `composables/useEntryType.ts` 一张查表里，不散在组件的 `v-if` 里。新增一种类型是加一行，不是改版式代码。

`travel` / `photo` 的领头图钉死 `aspect-ratio: 3 / 2`，因为一列混着 4:3 与 16:9 的图，边缘会参差。

### 13.3 字体：寒蝉正楷体（ChillKai）

用户选定，全站使用，包含正文。事实是从**字体文件本身**读出来的，不是猜的（fonttools）：

| 事实 | 对设计的影响 |
|---|---|
| 3.1 MB，**6763 个汉字 = 恰好 GB2312 的边界** | 现代日常汉字全覆盖；繁体、生僻字、许多人名字符**不在其中**，会在句子中间掉到 fallback，看起来像渲染故障。fallback 栈因此按「衬线优先」排，让替换尽量安静 |
| **只有一个字重**（400） | 全局 `font-synthesis: none`。合成粗体在毛笔字面上是涂抹，比没有粗体更糟。层级只能靠字号、颜色、留白 |
| **自然行高恰好 1.000**（零内建行距） | `line-height` 必须处处显式设置，否则行会贴在一起。`--leading-none` 定为 1.1 而不是 1.0，正是因为字体自己不留 |
| x-height 0.454（偏低） | 拉丁字母在同字号下显得比旁边的汉字小。用更大的基准字号补（`--text-base: 1.125rem`），而不是给拉丁另设样式 |

**CORS 是个真问题。** 用户提供的 CDN 不返回 `Access-Control-Allow-Origin`（GET、HEAD、OPTIONS 预检都试过，只有 `Timing-Allow-Origin`）。而 `@font-face` 的请求**永远**是 CORS 模式，所以直接跨域引用的结果是：3.2 MB 下载完，浏览器丢掉，页面用 fallback 渲染——**下载照付，字体不生效**。

解法是 Nitro 的 `routeRules` 反代到 `/fonts/chillkai.woff2`，变成同源请求，CORS 不再适用。代价是字体流量过 Nuxt 服务器一次。已验证：开发模式与 `nuxt build` 产物里 `url()` 都指向 `/fonts/chillkai.woff2`，取回的字节以 `wOF2` 开头，长度 3227920。

`font-display: swap`：3.1 MB 在慢网络上是好几秒，fallback 字面下可读的文字胜过正确字面下的空白。

### 13.4 值得记下的实现决策

**1. 配色只写一遍：`light-dark()`**

最初写成了两份——`[data-theme='lamp']` 一份、`prefers-color-scheme` 媒体查询里一份。那正是两套主题日后走偏的标准做法。改用 CSS `light-dark()` 后，每个颜色只有一行，`color-scheme` 决定取哪个参数。

**2. `html: false`，这是唯一的安全决策**

`utils/markdown.ts` 里 markdown-it 关掉了原始 HTML。**要打开它必须同时接一个真实的消毒器**（DOMPurify 或等价物），否则 `v-html` 就是一条 XSS 通道。

已跑 11 个注入向量：`javascript:` 链接、`data:text/html`、`<script>`、事件处理属性、`<iframe>`、`<img onerror>` 等。真实泄漏 0 个。

这里犯过一次错，值得记：第一版检测脚本用正则在整段输出里找 `javascript:` 字面量，报出 5 个「泄漏」。实际输出显示 markdown-it **根本没生成链接**——`[click](javascript:alert(1))` 保持为纯文本。误报来自检测脚本，不是渲染器。改成只在真实的 `href`/`src` 属性值里查危险 scheme、在真实元素上查事件属性后，11 个向量 0 泄漏。**在断言「发现漏洞」之前先看渲染出来的到底是什么。**

**3. 日期时区钉死 `Asia/Shanghai`**

`toLocaleDateString()` 用运行时时区。服务端（UTC）与读者（UTC+8）会对「一个深夜的时间戳属于哪一天」给出不同答案，那就是一次 SSR 水合不一致。`utils/date.ts` 里每个函数都显式传 `Asia/Shanghai`。

**4. 标题降级，保证 `h1` 唯一**

`renderMarkdown` 把正文里 h1–h5 各降一级，页面标题是唯一的 `h1`。因此作者写的 `##` 到达前台时是 `h3`。

配套的坑：`h5`/`h6` 原先既不在 margin reset 里、也不在标题字号规则里，于是作者写 `####`（降级后为 h5）会掉到浏览器默认样式。已补齐到 h6。降级在 h6 封顶（无匹配则不动），所以 `#####` 与 `######` 都落在 h6。

**5. 「粗体」与「斜体」都不能用常规做法**

单字重字面下 `font-weight: 700` 是空操作。所以 `.prose strong` 用**颜色加底色渐变**，`.prose em` 用 `text-emphasis: dot`（中文的着重号传统）。合成斜体的楷书看起来像 bug。

**6. 年份分组用「连续段」，不是 map**

API 已经排好序，对象键顺序是另一回事，不能依赖。所以按相邻元素是否同年累积。**前提是排序键与分组键一致**——这个前提当初不成立，见 13.5。

**7. 分页是链接，不是按钮**

每一页都是可分享、可抓取的真实 URL。`?page=1` 不生成，它是首页的重复 URL。不可用的方向用 `visibility: hidden` 保留位置，不用淡色：一个看得见又点不动的控件，任何颜色都解决不了——淡到像禁用就淡到读不了（原值实测 1.79:1）。

**8. 分类导航过滤掉空分类**

`entry_count > 0` 才出现在页脚。一个通向空页面的导航链接是条死路。但分类页自身仍要处理空列表：直接输入 URL 还是会到那里。

### 13.5 修掉的后端行为：排序键与显示键必须是同一个

**症状：**首页年份分组读出来是 **2026、2025、2024、2026**——2026 出现了两次。

**诊断：**把 API 返回顺序和实际显示的日期并排打出来：

```
slug                     happened_at                published_at                 显示年份
on-writing-things-down   2026-01-04T10:00:00+08:00  2026-08-25T20:55:19.942983   2026
faces-i-cannot-recall    2025-11-14T21:30:00+08:00  ...                          2025
convenience-store...     2025-09-19T02:14:00+08:00  ...                          2025
kafka-on-the-shore       2025-06-02T00:00:00+08:00  ...                          2025
bill-evans-alone         2024-12-11T23:00:00+08:00  ...                          2024
kyoto-sakura-in-water    2024-03-28T13:00:00+08:00  ...                          2024
kyoto-spring             —                          2026-08-25T17:49:01.215343   2026  ← 排在末尾
```

`kyoto-spring` 没有 `happened_at`，SQL 的 `NULLS LAST` 把它排到末尾；而前台 `entryDate()` 回落到 `published_at`，给它贴了个 2026。**按一个表达式排序、按另一个表达式显示，就会这样。**

**为什么不能在前端修。**在前端对当前页重新排序不解决问题：是后端决定哪些内容落在第 1 页，重排只是把断裂点从页面中间移到页面边界上。这个 bug 属于查询。

**改动：**

- `ListPublicEntries` 的 `ORDER BY` 改为 `COALESCE(e.happened_at, e.published_at) DESC, e.id DESC`
- 新增 migration **000005**，建表达式索引 `idx_entries_public_timeline`

`COALESCE` 在这些行上不可能是 NULL：`entries_published_at_check` 保证任何 `published` 内容都有 `published_at`。于是 `NULLS LAST` 这个特例一起消失了——**从不同角度看问题，让特例变成常规情况**，而不是再加一个条件判断。

索引不是可选项：Postgres 只在索引表达式与排序表达式文本一致时才用它排序，原来的 `(happened_at DESC)` 顶不了 `COALESCE(...)`。已用 `EXPLAIN` 确认走的是新索引。

**测试原来在断言旧契约。**`entry_integration_test.go` 的 `orders by happened_at, nulls last` 明确要求无日期的行排在末尾。它没红，因为 `make test` 在没有 `TEST_DATABASE_URL` 时跳过库测试——**`make test` 全绿不等于库层行为验过，要跑 `make test-integration`。** 该用例已改写：无日期行的 `published_at` 特意放在中间，于是「NULLS LAST」和「只按 published_at 排」两种错法都会被它抓住。

### 13.6 对比度：两处实测不合格，已改

配色写的是 OKLCH，肉眼估不出对比度，所以照 WCAG 公式算了一遍（OKLCH → sRGB → 相对亮度）。

改前：

| 前景 | 浅色主题 | 深色主题 | 判定 |
|---|---|---|---|
| `--c-ink` | 15.74 | 12.87 | AA |
| `--c-ink-muted` | 6.26 | 6.41 | AA |
| `--c-ink-faint` | **3.22** | **3.35** | **仅够大字号** |
| `--c-accent` | 5.60 | 6.61 | AA |
| `--c-line-strong` 当文字用 | **1.79** | **2.00** | **不合格** |

`--c-ink-faint` 有 13 处调用，承载的是日期、分类眉标、页脚计数、空状态——读者要读的东西，3.22:1 是真的不够。改为浅色 55%、深色 60%（实测 4.64:1 / 4.68:1，是这个纸色下刚过线的极值再收一点余量）。与 `--c-ink-muted` 的分离度是 1.35 / 1.37，两套主题一致。

**注释里写下了这条规则：比这更淡的需求属于边框 token，不属于文字 token。**

`--c-line-strong` 当文字用的两处已改：分页禁用态改成 `visibility: hidden`（见 13.4 第 7 条），错误页的大号状态码改用 `--c-ink-faint`。后者本来就免于 4.5:1（`aria-hidden` 且下方标题已复述），但「淡」得是能看见的淡。

### 13.7 验过什么

| 项 | 方法 | 结果 |
|---|---|---|
| 六条路由状态码 | curl | `/` `/?page=2` `/<slug>` `/categories/travel` 200；`/categories/nope` `/nonexistent-slug` `/draft-not-ready` 404 |
| **草稿与不存在的 404 不可区分** | 两份响应体逐字节比对（仅归一化 slug 本身） | 完全相同。后端的不可探测性穿过前台仍然成立 |
| 年份分组顺序 | 解析 SSR HTML | 2026 / 2025 / 2024，各一次 |
| Markdown 注入 | 11 个向量 | 0 泄漏（`href`/`src` 的 scheme 与元素事件属性两条口径） |
| 详情页结构 | 解析渲染结果 | `h1` 恰好 1 个；表格被 `.table-scroll` 包住；JSON-LD 与 canonical 都在 |
| 主题 cookie | 三种 cookie 值 | 无 cookie → 不写 `data-theme`；`ink`/`lamp` → 服务端渲染即带上；`bogus` 与 `"><script>` → 不写属性 |
| 字体反代 | curl + 构建产物 | 开发与构建产物都指向 `/fonts/chillkai.woff2`；取回 3227920 字节，magic 为 `wOF2` |
| 对比度 | 按 WCAG 公式算 OKLCH | 见 13.6 |
| 375px 宽度 | 算盒模型 | 内容列 327px；行内小图版式文字列 255px（≈14 字）；`.meta` 行实测约 108px，远未换行；`h1`–`h6` 已补 `overflow-wrap`，并用一条含长 URL 的真实标题验过 |
| 类型检查 / lint | `vue-tsc --noEmit`、`eslint` | 均无输出 |
| 生产构建 | `nuxt build` | 成功，2.75 MB（gzip 732 kB） |
| 后端 | `make test`、`make test-integration` | 全绿（本次两个都跑了） |

### 13.8 没验过什么

**界面没有在浏览器里看过。** 环境里没有浏览器自动化。上面所有结论来自 SSR HTML、构建产物、状态码与计算，**没有一条来自「看见它长什么样」**。具体而言，以下都只是代码上应该成立：

- 楷体实际渲染出来好不好看，fallback 掉字时有多明显
- 两套主题切换的观感，以及跟随系统时的初始状态
- 悬停交互（领头图 `scale: 1.02`、纯文字条目的墨线从左侧生长）
- 年份在侧栏 `position: sticky` 的实际吸附行为
- 375px 下的真实观感（盒模型算得出「不溢出」，算不出「好看」）

请在浏览器里核对：`/`、任意一篇详情页、`/categories/travel`，宽屏与 375px 各一遍。

**开发库里有我造的验证数据**，不是真实内容：分类 `读与看/reading`、`日常/daily`（另有 3B 留下的 `旅行/travel`），7 篇文章跨 2024–2026，其中 1 篇故意留作草稿以验证状态过滤。要清掉说一声。

## 14. Draft Autosave Foundation：浏览器验收 checkpoint（2026-08-26，BLOCKED）

本次从隔离工作树启动计划对应的后端和 `admin/`，并使用应用内浏览器做真实 UI 验收；没有用单元测试替代浏览器场景。

### 14.1 已完成的环境证据

| 项 | 命令/方法 | 实际结果 |
|---|---|---|
| migration | `cd backend && make migrate-up && make migrate-version`（加载本机既有开发环境变量） | `000006_entry_drafts_revision` 成功应用；版本 **6** |
| 后端 | `make run` | `/health/ready` 为 **200** |
| admin | Node 20.19.4 下 `npm run dev -- --host 127.0.0.1` | `http://127.0.0.1:5173/` 为 **200** |
| frontend | 端口检查 | 3000 已被主检出中预先存在的 Nuxt 进程占用；未终止或改动该进程 |

默认 Node 18.20.8 无法启动当前 Vite 8（`node:util` 缺少 `styleText` 导出）；切到本机已有的 Node 20.19.4 后 admin 正常启动。该进程版本差异没有修改项目文件。

### 14.2 阻断与未验证场景

应用内浏览器访问 `127.0.0.1:5173`、`localhost:5173` 均为 `ERR_CONNECTION_REFUSED`，而同一时刻 shell 对 admin 与后端健康端点均得到 200。为排除 loopback 网络隔离，曾仅对本次临时进程改为 `0.0.0.0` 监听，并从浏览器访问本机可路由地址；结果仍为同一拒绝连接。未改动任何配置文件，临时服务已停止。

因此无法安全登录或真实点击页面，以下浏览器场景均**未执行**，不能标为通过：创建两个空草稿、离线编辑后 reload 恢复、恢复网络同步、双标签 revision 冲突及其三项动作（服务端/本地覆盖/另存恢复草稿）、完整草稿发布、不完整草稿发布拦截。

没有生成有效应用截图；浏览器只呈现连接失败页，故无可用的 UI 截图或视口证据路径。待应用内浏览器可访问本机开发服务后，应从此 checkpoint 重新执行全部场景并补录实际请求状态。

### 14.3 浏览器恢复后的首轮验收（2026-08-26）

应用内浏览器现已可以访问 `http://localhost:5173` 并复用已登录会话。已实际确认：文章列表可加载；新文章编辑器可打开；标题、slug 与正文输入在等待保存后显示“已保存”；完整草稿可成功发布并显示“已发布”；空草稿点击发布会保留在草稿状态并显示标题必填错误。此次产生的验收内容为草稿 `验收草稿 A`（已发布）及一个空草稿，未删除任何数据。离线 reload、重连同步和双标签 revision 冲突专项仍待执行。

### 14.4 用户人工验收补充（2026-08-26）

用户反馈已自行完成并确认以下两项真实浏览器场景：断网编辑后 reload 恢复、恢复网络后的同步；双标签 revision 冲突及冲突解决动作。该结果作为用户人工验收记录，不附加自动化日志或截图声明。

## 15. Plan 2 沉浸式写作工作区：阶段一（Task 1–3，2026-08-26）

在独立 worktree `.claude/worktrees/immersive-writing-workspace`（分支 `worktree-immersive-writing-workspace`）执行，从 `main` 的 `cc1a610` 起步。主检出里未提交的 `admin/src/App.vue`（MilkdownProvider）和 `.superpowers/brainstorm/` 没有动过。

### 15.1 环境前置问题（先于本计划存在，未修复）

**admin 测试在 Node 20 下完全跑不起来。** `jsdom@30.0.1` 要求 `node: ^22.22.2 || ^24.15.0 || >=26.0.0`，并依赖 `undici@8`（`>=22.19.0`）；`undici` 调用 `worker_threads.markAsUncloneable`，该 API 在本机 Node 20.19.4 上不存在，于是 4 个 worker 全部启动失败，报 `webidl.util.markAsUncloneable is not a function`。

在 `main` 上同样失败，因此不是本计划引入的。本次改用本机已有的 Node 22.17.0 完成全部 admin 工作。注意 22.17.0 仍低于 jsdom 声明的 22.22.2 下限，能跑只是因为所缺的那一个 API 存在，并不在官方支持区间内。

**这件事没有在本计划里修**：它属于工具链基线，`engines`/`.nvmrc` 该钉在 22.22.2+ 还是把 jsdom 降级，是需要另行决定的事。

**另有一个先前存在的测试失败**：`TestRepositoryAdminReadsSeeEveryStatus/ordered_by_the_last_edit` 在设置 `TEST_DATABASE_URL` 时失败于 `version conflict`——它调用 `repo.Update` 时没带 `ExpectedRevision`，而 Plan 1 已把该字段变成必需。`make check` 不设该变量，数据库测试整体跳过，所以基线看起来是干净的。已在 `main` 上复现确认，同样未在本计划中修改。

### 15.2 Task 1：后台文章目录搜索

`GET /api/v1/admin/entries?q=` 按 `title`、`slug`、`summary` 做大小写不敏感匹配，**不搜正文**。服务签名改为 `ListAdmin(ctx, status, query, page, pageSize)`。

先写测试再实现，三层都有覆盖：service（假 store）、handler（HTTP 参数）、repository（真实 SQL）。

代码审查发现并修掉一个真实缺陷：**LIKE 元字符未转义**。`%`、`_`、`\` 会作为模式语法进入 ILIKE，实测确认打一个 `_` 会返回整个目录，标题「读完了 80% 的书」按名字搜不到。不是注入问题（值是绑定参数），是「搜索框回答了另一个问题」。转义放在 repository 层，因为它是这一层构造的 LIKE 模式的属性，而 trim 空查询是 service 的领域规则；这样也让内存假 store 的纯子串比较继续成立。

修复前后都验证过：先让新测试红（`_` 匹配 2 行而非 1 行），修复后绿。另加一条测试钉住「不搜正文」这个决定——在此之前删掉这条规则不会有任何测试反对。

审查提的查询长度上限没做：单用户已认证端点，且这些列本来没有索引，属于推测性加固。

### 15.3 Task 2：共享 Markdown 渲染

渲染实现移入本地包 `@alive/markdown`，前台与 admin 预览各 re-export。可执行代码与原 `frontend/utils/markdown.ts` 逐字节相同（排除注释后 diff 为空），22 条测试在移动之前写好，钉住的是前台已经发布的行为。

代码审查发现一个 **Critical**：**全新 clone 装不起来**。`npm install` 不会为 `file:` 依赖安装目标包自己的依赖，而包导出裸 `.ts`，`markdown-it` 必须从 `packages/markdown/` 往上找得到——它不在两个应用的 `node_modules` 里，也不在它们的 lockfile 里。本机两个 build 能过，只因为那个目录已经被手动装好了。把目录移走即可复现 `Cannot find module 'markdown-it'`。

处理方式：新增 `resolution.test.ts` 守住这条约束（移走 `markdown-it` 会让它失败，已验证），给包加上 `typescript` 与 `tsconfig.json` 让它能自检类型，并在 `architecture.md` 记下必须先跑 `cd packages/markdown && npm install` 的安装顺序。同时删掉 admin 里未被引用的 `markdown-it` 直接依赖——它解析出第二份独立钉版的渲染内核，正是共享包要消除的分叉风险。

没有把仓库改成 npm workspace。那会引入根 manifest、改变三个应用各自独立安装的现状，超出本计划范围，属于需要单独决定的结构变更。

### 15.4 Task 3：Alive UI 基元层

`admin/src/components/ui/` 下六个 Reka UI 薄封装 + `index.ts` + `ui.css`，业务组件不再直接 import `reka-ui`。

几个决定值得记：`UiDialog` 的 `title` 是必需 prop，因为 Reka 只在有 `DialogTitle` 渲染时才接 `aria-labelledby`，漏写就会得到一个「未命名」的对话框；`UiIconButton` 的 `label` 同理，目录收起等纯图标控件没有名字就只会被读成「按钮」。`UiMenu` 的条目走数据而非插槽，选中回传 id 不回传下标，避免列表被过滤后作用到错误条目。`UiToastRegion` 没有建在 Reka Toast 上，并且是命令式 API：一次播报是事件不是状态，做成 prop 会把 id 生成和过期计时推给每个调用方。

测试写完后做了变异验证：删掉 Reka 的 `triggerElement.focus()` 调用，焦点返回测试确实失败。过程中也发现 popover 的焦点返回断言原本是**假通过**——焦点从未进入浮层，所以「返回」无从验证；现在先塞一个可聚焦子元素再断言。

`vue-tsc` 还抓到两个 `vitest` 不会报的类型错误（未使用的 import、`aria-pressed` 的 `string | undefined` 不匹配），已修；顺带补了 `aria-pressed` 在「非开关」与「开关未按下」两种情形下的行为测试。

### 15.5 阶段一验证结果

| 项 | 命令 | 结果 |
|---|---|---|
| backend | `make check`（fmt + vet + test -race） | 通过 |
| backend + 数据库 | `TEST_DATABASE_URL=... go test ./internal/entry ./internal/entryhttp` | 除 15.1 那条先前存在的失败外全部通过 |
| packages/markdown | `npx vitest run` / `npm run typecheck` | 26 通过 / 通过 |
| admin | `npm test -- --run` / `npm run build` | 82 通过 / 通过 |
| frontend | `npm run typecheck` / `npm run build` | 通过 / 通过 |

浏览器验收留到 Task 7，因为 Task 4–6 才产出实际可看的写作界面。Task 3 的基元层目前还没有任何业务页面引用它，这在本阶段是对的。

### 15.6 Task 3 代码审查结论与修复（2026-08-26）

审查独立复现了我声称的变异测试（把 Reka 的 `triggerElement.focus()` 挖掉，焦点返回测试确实失败），并额外指出了几个真问题。已全部修掉，每一处都先用变异验证过测试真的会红：

**无障碍两处，都在 DOM 里核实过，不是推断：**

- **每个对话框都带着一个指向不存在元素的 `aria-describedby`。** Reka 的 `DialogContentImpl` 无条件把它设成一个生成的 id，而我从没渲染 `DialogDescription`。探针实测 `describedby=reka-dialog-description-v-1 targetExists=false`。现在加了可选 `description` prop，没有描述时用 Reka 官方的 `aria-describedby="undefined"` 退出。commit message 里为 `title` 写的那套理由，同等适用于这里，当时没做到。
- **`UiPopover` 的注释说错了 role，后果是真的漏洞。** 我写「popover 不会被当作命名区域播报」，但 Reka 实际渲染 `role="dialog"`（实测 `role=dialog label=null`）。也就是说没有 `label` 的 popover 就是一个未命名对话框——正是 `UiDialog` 的必需 `title` 要防的那件事。`label` 已改为必需。

**Toast 计时器两处缺陷：** 用 `Set` 存计时器时，提前 `dismiss` 不会取消它，计时器会继续空转到时长结束，`Set` 里还留着已不存在的 toast 的条目；改成按 id 存的 `Map`。另外 unmount 之后调 `publish` 没有防护，会往废弃的 ref 写并且再武装一个没人清理的计时器——它不抛错，只是静默泄漏，所以必须显式挡住。

**`ui.css` 里有两个字面颜色**，而这个文件的头部注释恰恰声明「这里只用语义 token」：primary 按钮文字的 `#ffffff` 和遮罩的 `rgb(27 26 24 / 32%)`。这正是注释警告的那种失败——主题化之后它们会变成唯一两个不跟随主题的元素，深色主题下遮罩会变成浅色蒙层，读起来像渲染故障。已提为 `--c-on-accent` 与 `--c-overlay` 两个新 token。

**三条测试是假通过的，已换掉或补上：**

- 「空状态不渲染任何文字」只断言 `textContent` 不含某串，那不管 region 元素在不在都成立——而「region 常驻挂载，好让屏幕阅读器在文字到达前就观察到它」正是这个组件的核心设计主张。现在给 region 套上 `v-if="toasts.length"` 会让测试失败（已验证）。
- 「Reka 类型不泄漏到 prop 表面」实际只断言了「一个按钮渲染出 button」，跟标题毫无关系。换成源码级守卫：`components/ui/` 之外任何文件 import `reka-ui` 就失败（在 `views/` 放一个探针文件验证过会红）。类型层面的那一半运行时无法表达，这是能真正回归的那一半。
- **计划要求的 popover 外部点击关闭测试当时没写。** 补上了，用 `pointerdown` 而不是 `click`——Reka 在 pointer-down 阶段就 dismiss，只测 click 的话，一个永不 dismiss 的组件也能通过。另补了 toast 自动过期与提前取消两条：把 `setTimeout` 的回调挖空会让其中两条失败。

顺带：`vue-tsc` 又抓到 `vitest` 不报的问题——那条源码级守卫要读文件系统，而 `tsconfig.app.json` 只给了 `vite/client` 类型。已在该配置加上 `node` 类型并写明理由（浏览器产物不受影响，`src` 下没有任何地方 import node 内置模块，真有的话 Vite 会直接报错）。

审查里两条没做，理由记下来：`UiButton` 的 loading 态目前只是变成 disabled，屏幕阅读器用户点了发布会得到一个安静且不可聚焦的控件——把它接到 toast 区域是 Task 5 发布面板的事，本任务不铺。以及 `UiMenu` 方向键测试只断言了移到第一项，第二次 ArrowDown 跳过 disabled 项这半没测；探针确认行为正确，留作后续。

### 15.7 当前状态与下次继续的入口

**在此暂停**（用户要求：Task 3 审查完成后停下，先记录文档）。**Task 4–7 未开始**，Plan 3 主题系统与 Plan 4 媒体上传按要求完全没有触碰。

分支 `worktree-immersive-writing-workspace`，worktree 位于 `.claude/worktrees/immersive-writing-workspace`，工作树干净，7 个 commit：

```
7194c4a fix: close accessibility and timer gaps in the UI primitives
e8cd4cb docs: record Plan 2 stage one progress
f845608 feat: add accessible Alive UI primitives
8f14d76 fix: make the markdown package installable from a clean clone
dbe44a4 refactor: share markdown rendering
eb6e44c feat: search the admin article directory
```

主检出里未提交的 `admin/src/App.vue`（MilkdownProvider）与 `.superpowers/brainstorm/` 始终没有改动。

**最终验证（全部通过）：** backend `make check` 无失败；`packages/markdown` 26 测试 + typecheck；admin 85 测试 + build；frontend typecheck + build。

**下次继续前需要知道的两件事：**

1. **admin 的一切必须在 Node 22 下跑**（见 15.1）。`export PATH="$HOME/.nvm/versions/node/v22.17.0/bin:$PATH"`。Node 20 下 vitest 一个测试都起不来。
2. **`packages/markdown` 的安装顺序是硬要求**：先 `cd packages/markdown && npm install`，否则 admin 与 frontend 的 typecheck/build 都会失败在 `Cannot find module 'markdown-it'`。`resolution.test.ts` 守着这条。

**Task 4 的起点：** 路由要加第二个顶层 `WritingLayout` 记录（`/entries` 带 `new` 与 `:id` 子路由），与现有 `/entries` 库列表并存而不嵌套；`admin/src/views/EntryEditor.vue` 目前 1047 行，是待拆的主体；Plan 1 的 `save-coordinator.ts` / `recovery-store.ts` / `useEntryAutosave.ts` 已就位待接入；Task 3 的基元层目前还没有任何业务页面引用，Task 4 是第一个消费者。

## 16. Plan 2 Task 4：沉浸式写作壳与文章目录（2026-08-27）

已提交为 `c86b19c`。**审查已完成，但审查发现的问题一处都还没修**——下次会话的第一件事就是这个，详见 16.4。

### 16.1 交付内容

新增 `admin/src/layouts/WritingLayout.vue`、`admin/src/components/writing/{ArticleDirectory,WorkspaceHeader}.vue`、`admin/src/stores/writing.ts`、`admin/src/router/routes.test.ts`；改 `router/index.ts`、`AdminLayout.vue`、`EntryEditor.vue`（+268/−307）。

三个决定值得记：

**路由是两条平级顶层记录，不是嵌套。** 嵌套会把导航栏和画布一起挂上，而这个工作区存在的意义就是取代那个布局。但两条记录都用 `/entries` 前缀，这带来一个真陷阱：父路径即使没有空路径子路由也会拿到自己的 matcher，所以裸 `/entries` 被两条记录以相同分数命中，Vue Router 靠**声明顺序**破平局。写作记录放前面时，`/entries` 会渲染写作壳套一个空画布，而不是文章库。我用真 router 独立验证过这个行为，`routes.test.ts` 钉住了顺序。

**store 只持有三件事**：`directoryOpen`、`activeEntryId`、`searchQuery`。文档内容归 Milkdown、经保存协调器上行；把任一份放进全局 store 就等于给一个值两个所有者，输的那个是后跑的那个——这样被覆盖掉的文字在 UI 上是看不见的，所以这道防线必须是结构性的。

**`activeEntryId` 由编辑器写入，不从路由读。** `/entries/new` 路径里没有 id：先建空草稿、再 `router.replace`。读路由的话，目录会在那个往返期间一直没有高亮项。

**flush gate 走 provide/inject 而不是 store**：它是一个指向已挂载组件协调器的活引用，放进 Pinia 会让「有生命周期的函数」看起来像「可序列化的值」。

### 16.2 我亲自验证的结果

| 项 | 结果 |
|---|---|
| admin | 154 测试通过 / `npm run build` 通过 |
| backend | `make check` 零失败 |
| frontend | typecheck + build 通过 |

计划硬约束逐条核对过：store 边界、`reka-ui` 没泄漏到业务组件（有测试守着）、14rem 栅格与折叠归零、250ms 防抖、`page_size=20`、`无标题草稿` 兜底、未同步点来自 `EntryRecoveryStore.list()`、底部保存按钮已移除。

**变异测试**：防抖 250→0 让防抖测试红；删掉 `await flushActiveEntry()` 让两条测试红。这些测试不是摆设。

### 16.3 子进程编造输出，两次

这一轮 Task 4 的实现和审查都派给了子进程，因为我的 Bash/Read 工具在会话中段开始持续返回空。两次都出现了**编造**：

1. 实现子进程报告「tree clean at `c3e8f0a`，154 测试通过」。`c3e8f0a` **不存在**，当时 HEAD 还是 `c8c3bff`，什么都没提交，工作树是脏的。代码和 154 通过是真的，「已提交」是编的——那个 commit 是我工具恢复后自己做的。
2. 审查子进程发来的报告以「上面的报告」开头，只给了 findings 11、12 和总结。我要它重发 1–10，它回复承认：**前面那份报告从未存在过**，它引用了自己没写过的内容。1–10 是它事后补写的（证据是真的，编号和文字是新的）。

两次它都提到自己的工具输出有乱码，跟我这边返回空大概是同一现象。**结论：子进程报的 SHA、测试数、"已提交" 一律要用自己的工具核对，不能转述。** 我核对了，所以 16.2 的数字是实的。

### 16.4 审查发现，全部未修（下次会话从这里开始）

审查独立复现了我做的变异测试，并另外找出以下问题。我只结构性确认了第 1 条（`.ui-dialog__content` 确实没有 `max-height`，`ArticleDirectory` 确实是 `height: 100%; overflow-y: auto`）；**其余各条的像素测量与 `elementFromPoint` 结果我没能亲自复核**（工具反复失效），修之前应当先在真浏览器里复现。

**Critical 3 条：**

1. **手机抽屉把自己的内容裁到屏幕外，且无法滚动。** `.ui-dialog__content` 没有 `max-height`，是居中 fixed 盒；`ArticleDirectory` 的 `height: 100%` 在 auto 高度的 `.ui-dialog__body` 里解析不出约束，`overflow-y: auto` 因此永不激活。声称在 375×700 实测：对话框高 1556px、`top: -428`，只有 3 条最近文章进得来，品牌行/新建文章/搜索框全在 `top: 0` 之上不可达。没有任何测试断言抽屉高度或可滚动性。
2. **375px 冲突态下，恢复动作不可达，且与「发布」重叠。** `WorkspaceHeader` 的 `grid-template-columns: minmax(0,1fr) auto minmax(0,1fr)` 加 `.status` 的 `overflow: hidden`，声称状态轨需 373px 只得 327px，裁掉第三个按钮「另存为恢复草稿」；同时右簇被挤到 0px 而需 146px，向左压到状态区上。对被裁按钮中心点做 `elementFromPoint` 返回**发布**。这条在防数据丢失的唯一路径上。`offline` 态同样退化，程度较轻。
3. **被守卫否决的文章切换 rejection 未处理，静默失败。** `openEntry` await `router.push()` 没有 try/catch；编辑器的 `onBeforeRouteUpdate` 在 status 非 `saved`（离线/错误/冲突）时返回 `false`，Vue Router 于是 reject。离线状态下点另一篇文章，抽屉不关、无提示、什么都不动。`createArticle` 有 try/catch，所以这个缺口是 `openEntry` 独有的。

**Important 4 条：**

4. **新组件的触摸目标远低于 44px。** `ui.css` 的 `@media (pointer: coarse)` 只覆盖 `.ui-button`/`.ui-icon-button`/`.ui-menu__item`，新业务组件用的是裸元素。声称实测：`.group-toggle` 28.8px、搜索框 21.5px、页脚链接 22.4px、`.status-action`（含冲突按钮）26.5px。文章行 50.4px 是合格的。
5. **`WorkspaceHeader.test.ts:56-65` 部分假通过。** 它给每个状态各挂一个新 wrapper 再断言 `aria-live === 'polite'`，只证明了模板里有这个属性。给状态 div 加 `:key="saveStatus"`（让 live region 每次状态变化都重挂——正是注释描述的那个 bug）之后 17 条测试全绿。`v-if` 变异**能**被抓到，所以它有部分价值。修法是用一个 wrapper 配 `setProps` 跨状态，断言节点标识不变。
6. **目录行挂载后永不刷新。** `loadRecent`/`loadUnsynced` 只在 `onMounted` 跑。桌面列不卸载，所以：给空草稿打上标题、自动保存写入服务端，那一行整个会话都还显示「无标题草稿」；未同步点也只反映挂载时刻的状态，进入冲突后要重载才看得到标记——标记在最需要它的时候最不准。`createArticle` 会把新记录 splice 进 `recent`，所以缺口专指挂载后对既有行的变更。
7. **任何跨越视口断点都会丢弃用户显式的折叠。** `applyMatch` 无条件 `setDirectoryOpen(!matches)`。桌面宽度下手动收起目录，跨过 48rem 再回来，它自己又开了。注释论证了窄向是有意的，宽向则覆盖了用户的明确选择。现有测试钉住了当前行为，所以改它是个需要决定的事，不是默默修。

**Minor 3 条：** 8. 删除菜单项渲染成红色危险样式但点击无反应（`handleHeaderAction` 只处理 `unpublish`/`archive`）——`handlePreview` 是干净的空函数，符合注释；`delete` 不是，它是个会吞掉点击的可见破坏性控件，应当先从菜单里去掉。9. `EntryEditor.vue` 里 `.badge*` 三条 CSS 已成死代码。10. 五处 `.element).toBeTruthy()` 等于没断言——`wrapper.get()` 选不到就已经抛了。

审查自己的总结值得抄在这里：这三个 critical 有共同根因——**这个工作区赖以成立的布局属性（不抖动、抽屉可用、恢复动作可达）恰好是 jsdom 观察不到的那些**，而源码级替代断言检查的是声明而不是结果。固定高度状态槽本身是有效的，包着它的栅格不是，而整个测试套件里没有一条能分辨这个差别。

### 16.5 下次会话的入口

**分支** `worktree-immersive-writing-workspace`，worktree 在 `.claude/worktrees/immersive-writing-workspace`，工作树干净，HEAD `c86b19c`，共 8 个 commit。主检出里未提交的 `admin/src/App.vue`（MilkdownProvider）与 `.superpowers/brainstorm/` 始终没碰。

**两个会立刻咬人的环境事实**（详见 §15.1）：

1. admin 的一切必须 `export PATH="$HOME/.nvm/versions/node/v22.17.0/bin:$PATH"`，Node 20 下 vitest 一个测试都起不来。
2. `packages/markdown` 必须先 `npm install --legacy-peer-deps`，否则两个 app 的 build 都失败在 `Cannot find module 'markdown-it'`。

**建议顺序：** 先修 16.4 的 Critical 1–3（都在这次提交的代码里，且第 2 条在数据丢失路径上），修的时候在真浏览器 375px 下复现和验证——这三条 jsdom 抓不到。然后再进 Task 5。

**Task 5 的起点：** 计划文件第 315 行起；规格 5.4（抽屉字段）、5.5（发布面板）、11（错误处理）。Task 4 留了三个故意惰性的钩子给它填：`EntryEditor.vue` 的 `handlePreview()` 空函数、`WorkspaceHeader.vue` 菜单里 `settings` 与 `delete` 两个 id（`handleHeaderAction` 未处理，各有注释标明属于 Task 5）。要搬进抽屉的七个字段已经聚在 `EntryEditor.vue` 一个 `.fields` 块里：slug、type、category、summary、cover_url、happened_at、visibility；标题和 Milkdown 编辑器留在画布上。

Task 6、7 未开始。Plan 3 主题系统与 Plan 4 媒体上传按要求完全没有触碰。

## 17. Task 4 Critical 1 的浏览器实测（2026-08-27，进行中）

**代码没有任何改动。** HEAD 仍是 `bda68a0`，工作树干净。这一节只记录一次实测结果，因为它和 §16.4 审查声称的数字**不一致**，下次动手前需要知道这件事。

### 17.1 做法

在真浏览器里搭了一个一次性 harness（`admin/repro.html` + `repro.ts`，已删除），挂载**真实组件与真实样式表** —— `WorkspaceHeader`、`ArticleDirectory`、`UiDialog`，加载 `style.css` 与 `ui.css`，API 在 `fetch` 层打桩返回 20 条（真实 `page_size`），这样目录自己的加载代码原样运行。视口 375×700。

harness 需要 `VITE_API_BASE_URL`（客户端没有它会抛错），临时写了 `admin/.env`（已被 gitignore，且已删除）。

### 17.2 实测数字 vs 审查声称

| 项 | 审查声称 | 我实测 |
|---|---|---|
| 对话框高度 | 1556px | **1482.9px** |
| `top` | −428.2 | **−391.4** |
| `bottom` | 1128.2 | **1091.4** |
| 完整可见的文章行 | **3** | **12** |
| `.ui-dialog__content` 的 `max-height` | 无 | **`none`**（确认） |
| 可滚动祖先 | 无 | **`null`**（确认） |
| 搜索框可达 | 不可达 | **不可达**（`top: −231.3`） |

**缺陷是真的，量级不是。** 结构性根因完全确认：`.ui-dialog__content` 的 `max-height` 计算值是 `none`、`overflow` 是 `visible`，从文章行往上找不到任何可滚动祖先（`scrollableAncestor: null`）。所以对话框高 1482.9px 撑在 700px 视口里，上下都溢出，且**没有任何方式滚动**。搜索框在 `top: −231.3`，品牌行和「新建文章」在它之上，全部不可达 —— 这部分和审查一致。

但「只有 3 条进得来」是错的，实测 12 条完整可见。差异原因大概是审查用的行高假设与真实渲染不同（它自己也说过测量是在它的环境里做的）。**修复的紧迫性不变，但描述缺陷时要用 12 这个数。**

这也再次印证 §16.3 的结论：子进程报的具体数字必须自己复核。这次三个数量级里有一个是错的。

### 17.3 Critical 2、3 仍未复核

被打断前只做完了 Critical 1。**Critical 2（375px 冲突态下「另存为恢复草稿」被裁、其像素点命中「发布」）和 Critical 3（`openEntry` 未捕获路由守卫 reject）我仍然没有亲自复现过。** 修之前应当用同样的 harness 手法验证，尤其是第 2 条的 `elementFromPoint` 结论 —— 那是整条发现的要害，而它来自一个已经在数量上出过错的来源。

复现 Critical 2 的要点：harness 里 `WorkspaceHeader` 的 `saveStatus` 设为 `'conflict'`，`conflict` 插槽放三个 `.status-action` 按钮（真实内容见 `EntryEditor.vue:545-572`：载入服务端 / 覆盖服务端 / 另存为恢复草稿），375px 下量三条栅格轨道的实际宽度与需求宽度，再对被裁按钮中心点做 `document.elementFromPoint`。

### 17.4 下次继续

入口不变，见 §16.5。顺序建议：

1. 复现并修 Critical 1（根因已确认：`.ui-dialog__content` 缺 `max-height`，抽屉内无可滚动容器）
2. 复现 Critical 2、3，确认后再修
3. 修完在真浏览器 375px 复验，然后进 Task 5

注意 harness 是一次性的，已删除；重建时记得 `admin/.env` 里给 `VITE_API_BASE_URL`，用完连 `.playwright-mcp/` 一起删掉，别提交进去。

## 18. Critical 1–3 修复与 Task 5 起步（2026-08-27）

### 18.1 Critical 1–3：复现、修复、复验

按要求先在真实浏览器 375×700 复现，再动 Critical 2、3。Critical 1 的审查数字再次证明不准确：修复前实际对话框高度不是 1556px 而是 1482.9px，但根因一致——`.ui-dialog__content` 没有高度约束，目录没有可滚动祖先，搜索框在视口上方不可达。

修复如下：

- `.ui-dialog__content` 增加 `max-height: min(40rem, calc(100vh - 2 * var(--space-4)))` 与 `overflow-y: auto`。
- 375px 冲突态先实测确认：三枚恢复按钮确实被右侧栅格挤出且命中其他按钮。移动端 header 改为两行，冲突状态占整行并允许按钮换行；最终真实测量三枚按钮均完整位于视口内，`elementFromPoint` 命中自身。
- 真实点击目录文章时确认 Vue Router 的守卫否决在此处可能以 resolved NavigationFailure 返回，也可能由 flush gate reject。`openEntry` 现在捕获两种情况，保留当前文章/抽屉并显示“无法切换文章，当前编辑状态未改变。”。

目标 worktree 的 `App.vue` 同步补上 `MilkdownProvider`。这是主检出里原本未提交、且明确未触碰的文件；主检出内容没有修改。没有该 provider，真实编辑页会在 Milkdown setup 阶段报 `editorFactory` 注入缺失，无法进行后续浏览器验收。

### 18.2 验证与代码审查记录

先红后绿的新增测试覆盖：对话框滚动约束、移动端冲突行布局、flush rejection、resolved NavigationFailure、App 的 Milkdown provider，以及发布检查纯函数。admin 全量结果：**162 tests passed，build passed**。`git diff --check` 无输出。

本轮没有可用的独立审查子进程工具，因此按审查清单对 diff、测试变异点、浏览器几何和命中元素逐项复核；未声称把这一步等同于独立审查。Task 5 目前只完成 `publish-checks.ts` 与其 2 条测试，设置抽屉、发布面板、预览尚未开始；Task 6、7 未开始。Plan 3 主题系统与 Plan 4 媒体上传没有触碰。

## 19. Task 5 面板与 Task 6 命令基础（2026-08-27，进行中）

### 19.1 Task 5：已实现并验证

- 新增 `ArticleSettings.vue`：slug、类型、分类、摘要、封面 URL、发生时间、可见性，以及显式确认删除；每次字段变化只向 `EntryEditor` 发局部 PATCH 字段。
- 新增 `EntryPreview.vue`：使用共享 `@alive/markdown` 渲染本地标题和正文，支持桌面/手机预览宽度。
- 新增 `PublishPanel.vue`：阻塞项、提醒确认、可见性文案和 revision-aware 发布事件。
- `EntryEditor.vue` 已接通 settings、preview、publish 菜单和面板，发布仍由原 autosave/coordinator 与 transition 逻辑执行。

验证：Task 5 相关测试通过；包含 Task 6 基础测试的 admin 全量 **174 tests passed**，`npm run build` 通过。Task 5 没有可用的独立审查子进程工具，因此没有把自审称作独立审查；已用测试、类型检查和构建复核。

### 19.2 Task 6：已开始

- 新增并测试 `editor-commands.ts` 命令注册表，覆盖二级/三级标题、列表、引用、代码块、分隔线。
- 新增并测试 `slash-menu.ts` 的 slash 查询识别与命令筛选。
- 新增并测试 `selection-toolbar.ts` 的非空选区显示规则和五个动作定义。

尚未完成 Milkdown 插件实际挂载、命令执行到 ProseMirror、选区工具条、链接快捷键及 MarkdownEditor 的完整集成；Task 7 尚未开始。Plan 3 主题系统与 Plan 4 媒体上传没有触碰。

### 19.3 全仓验证与浏览器缺口

- backend `make check` 通过。
- frontend 在 Node 22.17.0 下 `npm run typecheck` 与 `npm run build` 通过；Node 18 会在 Nuxt CLI 的 `node:util styleText` 处失败。
- admin 在 Node 22.17.0 下全量 **175 tests passed**，build 通过。
- 真实浏览器已复用现有登录会话进入 `/entries/18`：确认元数据已从主画布移入设置面板，设置字段 label 可定位，预览面板可打开，发布面板会显示阻塞项/提醒项。
- 375×700 实测：页面 `scrollWidth=375`；设置面板宽度为 375px 且内部可滚动；预览框左右边界为 24px/351px，无横向溢出。浏览器临时视口覆盖已清除。未创建临时账号。

## 20. Task 6 上下文编辑器控件（2026-08-27，进行中）

### 20.1 实现与测试

- `editor-commands.ts` 提供 slash 命令注册表，支持中文标签和英文关键词筛选，并把命令执行与 slash 文本删除绑定。
- `slash-plugin.ts` 接入 Milkdown `slashFactory` / `SlashProvider`，在段落末尾的 `/` 上下文显示 `UiMenu`，支持键盘导航、回车执行和代码块隐藏。
- `selection-toolbar.ts` 定义粗体、斜体、删除线、行内代码、链接五个非空选区动作；`MarkdownEditor.vue` 按真实选区显示工具条。
- `MarkdownEditor.vue` 接入链接 tooltip、Cmd/Ctrl+K 快捷键、Milkdown history，并新增 `ready(controller)` 事件；原有 Markdown `update` 输出契约保持不变。
- 新增链接快捷键、slash 插件和 MarkdownEditor 配置测试。

验证：admin 全量 **178 tests passed**，Node 22.17.0 下 `npm run build` 通过。

### 20.2 真实浏览器复验

- 复用现有登录会话进入 `/entries/18`，输入 `/` 后确认 slash 菜单出现；按 Enter 选择“二级标题”后确认 ProseMirror 节点转换为标题。
- 选中文本后确认“文字格式”工具条出现，包含五个动作。
- 选中文本后按 Cmd/Ctrl+K，确认链接编辑浮层出现并聚焦 `Paste link...` 输入框。
- 发现并修复 slash 命令转换标题后 slash 文本删除失效的问题；删除逻辑现在同时识别段落和标题节点。

本轮没有可用的独立审查子进程工具，因此没有把自审称作独立审查；已用失败测试、全量测试、类型检查、构建和真实浏览器行为复核。Task 6 已提交为 `5b60c27`，Task 7 尚未开始。Plan 3 主题系统与 Plan 4 媒体上传没有触碰。

## 21. Task 7 响应式与可访问性验收（2026-08-27，进行中）

### 21.1 自动化检查

- backend `make check` 通过。
- admin 在 Node 22.17.0 下全量 **180 tests passed**，`npm run build` 通过。
- frontend 在 Node 22.17.0 下 `npm run typecheck` 与 `npm run build` 通过。
- 新增 ArticleSettings Escape 关闭测试；设置抽屉关闭后恢复打开它的“更多操作”按钮焦点。

### 21.2 真实浏览器验收

- 375×700：设置抽屉左右边界为 0/375，内部 `scrollHeight=814`、`clientHeight=700`，页面 `scrollWidth=375`；按 Escape 关闭后焦点返回“更多操作”。
- 链接编辑浮层按 Escape 后 `data-show=true` 数量为 0；编辑器选区工具条和 slash 菜单已在前一阶段验证。
- 1024×768、768×1024、375×700 三种尺寸的 `document.scrollWidth` 均等于视口宽度，无横向溢出。

本轮没有可用的独立审查子进程工具，因此没有把自审称作独立审查；Task 7 的自动化、重点响应式验收和最终全量复核均已完成。浏览器联调产生的 `/entries/18` 测试内容未擅自清理。Plan 3 主题系统与 Plan 4 媒体上传没有触碰。

## 22. 移除 slash 菜单并固定编辑器占宽（2026-08-27）

- 根据验收反馈，移除编辑器内 slash 触发菜单及其命令注册表、Milkdown slash 插件和相关测试；保留选区格式工具条与链接编辑能力。
- 同步更新实施计划，明确 Task 6 不再包含 slash 菜单；`UiMenu` 文档不再把 slash 菜单作为使用场景。
- `MarkdownEditor` 外壳、Milkdown 容器和 ProseMirror 内容区显式设置 `width: 100%`、`min-width: 0`，并增加 `box-sizing`、长内容换行和稳定的最小编辑高度，避免编辑器随内容宽度自适应。
- 新增测试确保 slash 菜单未挂载，并确保编辑器使用稳定的全宽写作面。

验证：admin 定向测试和构建通过；真实浏览器已复核桌面端与 375px 布局，确认无 slash 菜单、编辑器稳定占宽且无横向溢出。Plan 2（Task 1–7）至此完成；未触碰 Plan 3 主题系统或 Plan 4 媒体上传。
## 23. Plan 3：共享站点主题系统（2026-08-27，Task 1–5 完成）

- 已建立 `@alive/theme` 本地包，提供 `THEMES`、`ThemeName`、`isThemeName` 和 `resolveTheme`。
- 已定义 `ink`、`lamp`、`codex-lavender` 三套共享语义 token，包含字体、表面、正文、边框、强调色、危险色和遮罩色。
- admin 与 frontend 的 manifest 和 lockfile 已接入本地主题包。
- TDD 验证：先确认主题包入口缺失导致测试失败，再实现后 3 tests passed。
- Task 2 已完成：新增 `site_settings` 单例表、`000007` migration、sqlc 查询、`internal/site` domain service 和 repository，以 revision 保证主题默认值更新的乐观并发控制。
- TDD 验证：服务测试先因 `internal/site` 实现缺失而失败，实现后 4 tests passed。
- Task 3 已完成：新增公开 `GET /api/v1/site`、登录保护的 `PATCH /api/v1/admin/site`，并在 router 与 server 启动装配中接入；API 文档已同步。
- TDD 验证：handler 测试先因 `NewHandler` 缺失而失败，实现后 sitehttp、router、server 定向测试通过。Task 4–6 尚未开始。Plan 4 媒体上传没有触碰。
- Task 4 已完成：后台主题 store 支持预览、取消预览和 revision-aware 保存；目录底部接入 ThemePicker；共享主题 CSS 只加载一次，后台保留布局 token；开发环境已代理 ChillKai 字体。
- TDD 验证：ThemePicker 源码测试先因实现缺失而失败，实现后主题 store 与组件测试共 5 tests passed；Node 22 下 admin build 通过。构建仅提示字体绝对路径在运行时解析及既有大 chunk warning。
- Task 4 代码审查：当前环境没有可用的独立审查子进程，因此未将自审称作独立审查；已通过逐文件审阅、定向测试、类型检查和生产构建复核。Task 5–6 尚未开始，Plan 4 媒体上传没有触碰。
- Task 5 已完成：frontend 通过 `useSiteSettings` 复用 SSR 站点设置请求；`useTheme` 使用共享 resolver，让有效访客 Cookie 覆盖站点默认，清除 Cookie 后重新跟随站点；主题菜单提供“跟随站点”和三套主题。
- 前端旧 tokens 中的字体、颜色和主题块已移除，正文改读共享 `--font-prose` 与 `--c-prose` 语义 token；Nuxt 全局 CSS 顺序已调整为共享主题优先。
- TDD 验证：优先级测试先因 `resolveVisitorTheme` 不存在而失败，实现后 4 tests passed；Node 22 下 frontend typecheck 与 SSR build 通过。构建仍仅提示字体绝对路径运行时解析。
- Task 5 代码审查：当前环境没有可用的独立审查子进程，因此未将自审称作独立审查；已通过逐文件审阅、失败测试回红、测试、类型检查和 SSR 构建复核。Task 6 尚未开始，Plan 4 媒体上传没有触碰。
- Task 6 已完成：backend `make check` 通过；admin 全量 183 tests 与 build 通过；frontend typecheck、SSR build 通过。
- 真实浏览器：frontend SSR 首屏确认 `<html data-theme="ink">`；菜单确认可切换 `灯下`、`Codex Lavender`，并可用“跟随站点”清除访客选择；1440px 与 375px 均无横向溢出，375px 截图确认主题菜单和正文可读。
- 对比度证据（基于实际共享 hex token 与对应纸面背景计算）：纸墨正文/弱化/淡化/链接为 16.53/6.42/5.10/6.92；灯下为 15.17/8.06/6.96/9.28；Codex Lavender 标题/正文/弱化/淡化/链接为 7.65/5.82/5.56/5.51/6.08，均达到普通文字 AA。Codex Lavender 强调色由原 3.69:1 的 `#9a6bdc` 调整为 `#7a45b5`，保持色相并满足要求。
- 后台真实浏览器受环境限制：本工作树使用 5183 端口时，运行中的 backend 对 `Origin: http://localhost:5183` 返回 403 CORS；未擅自重启或修改运行中的服务，故未伪造后台主题保存验收。实现与自动化测试已覆盖后台预览/保存路径。
- Task 6 代码审查：当前环境没有可用的独立审查子进程；已通过逐文件审阅、全仓检查、真实浏览器检查和对比度复核。Plan 3 已完成，Plan 4 媒体上传没有触碰。

## 24. 编辑器写作面收束（2026-08-27）

- 标题字段移入“文章设置”，主画布不再重复显示标题输入框，也移除了“正文”标签和底部 Markdown 说明。
- 删除操作移至右上角，并保留显式二次确认；未在浏览器验收中实际删除文章。
- 编辑器外壳去除边框，改为稳定的全宽写作面；保留内容换行和最小编辑高度。
- 标题节点支持点击或键盘 Enter/Space 展开、收起其下级内容；折叠仅影响当前视图，不改变 Markdown 源文本。补充 MutationObserver，覆盖 Milkdown 延迟插入内容的真实挂载时序。
- 根据后续验收反馈，移除 Milkdown 底部链接预览/“Paste link...”浮层、Cmd/Ctrl+K 链接快捷键和选区工具条中的“链接”动作，避免编辑区出现无关链接入口。
- TDD：新增标题设置、头部删除、布局和标题折叠测试；先确认缺失实现的测试失败，再完成实现。
- 验证：admin 全量 **189 tests passed**，Node 22.17.0 下 build 通过，`git diff --check` 通过；浏览器已确认标题设置、右上角删除确认、375px 无横向溢出。浏览器后续重载在连接超时前未能完成新增折叠属性的第二次实测，故该部分以自动化测试和挂载时序修复为主要证据。
- 本轮没有可用的独立审查子进程工具，因此没有把自审称作独立审查；已完成逐文件审阅、失败测试回红、全量测试、类型检查、构建和可用的真实浏览器验收。Plan 3 主题系统与 Plan 4 媒体上传没有触碰。

## 25. 编辑器层级交互与目录栏调整（2026-08-27）

- 用户验收后放弃编辑器内标题层级展开/缩起功能，移除对应事件监听、箭头样式、辅助模块和测试，避免保留不可靠的半成品交互。
- 用户进一步确认后台不需要文章预览，移除编辑页预览入口、预览面板及其状态和测试；主题设置中的主题预览属于独立功能，保留不变。
- 文章目录滚动条改为细窄、低对比度样式；桌面端增加可访问的垂直分隔拖拽手柄，宽度限制在 220px–420px，并持久化用户选择，键盘支持方向键、Home 和 End。
- TDD：目录栏尺寸约束测试保留并验证；标题折叠相关测试随功能撤销一并移除。
- 本轮最终验证以撤销后的全量测试、Node 22 构建和 `git diff --check` 为准；浏览器已确认错误覆盖消失、粘贴链接不再出现、拖拽手柄可用。当前环境没有可用的独立审查子进程工具，因此未将自审称作独立审查。

## 26. 前台文章详情阅读列调整（2026-08-27）

- 前台文章详情从共享 `--measure` 阅读宽度调整为 48rem，并让标题、元信息、封面和正文统一居中；窄屏仍通过 `width: min(100%, 48rem)` 保持流式收缩。
- Node 22 下 frontend typecheck、SSR build 和 `git diff --check` 通过；真实浏览器在 1280px 视口确认文章列与正文均为 768px，左右边界对称且无横向溢出。

## 27. 前台文章上下篇导航（2026-08-27）

- 文章详情底部增加“上一篇 / 下一篇”快捷链接，复用公开文章列表的最新优先顺序；边界文章只显示可用方向，正文请求或导航列表失败不会互相影响。
- 导航链接使用文章 slug，并设置 `rel="prev"` / `rel="next"`；导航计算已通过 2 条单元测试，Node 22 下 frontend typecheck、SSR build 和 `git diff --check` 通过。
- 真实浏览器已确认当前文章显示两个导航入口，分别指向相邻文章，页面无横向溢出。

## 28. 前台底部导航与主题菜单交互优化（2026-08-27）

- 文章返回入口移至标题上方，改为轻量的“全部文章”链接；上下篇导航不再渲染空占位，首篇或末篇只显示可用方向。
- 主题切换按钮增加主题色标记、展开箭头和轻量展开动画；菜单支持点击外部区域关闭，也支持 Escape 关闭，并遵循 `prefers-reduced-motion`。
- 真实浏览器已验证主题菜单展开后点击正文区域会关闭，菜单内部选择不会误关闭，页面无错误覆盖层；frontend typecheck、SSR build 和 `git diff --check` 通过。
