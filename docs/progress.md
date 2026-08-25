# Alive 施工进度

最后更新：2026-08-25

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

**Stage 1（users + auth）与 Stage 2 的后端 Entry API 均已完成**：建、改、软删、publish / unpublish / archive、前台列表与详情、Admin 列表与详情，共 10 个端点。`frontend/` 与 `admin/` 尚未开始，目前只有 `backend/`。

Stage 2 剩下两件不属于本次范围的事：`categories` 表（migration `000004`）与 `visibility=unlisted` 的独立查询，见 3.1。

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
| PostgreSQL | 18.6 运行中（brew services），库 `alive` 与 `alive_test` 均在**版本 3**（表：`users` `sessions` `entries`） |
| 开发库账号 | `owner` / `alive-dev-password-2026`，display_name `P30 Huiwei` |
| `sqlc` / `golang-migrate` | 已装 |
| `GOPROXY` | 必须走 `https://goproxy.cn,direct`。`proxy.golang.org` 从此网络连不通，Makefile 已内置，但**在 make 之外手动跑 `go get` 需自己带上** |
| 端口检查 | 用 `lsof -iTCP:8080 -sTCP:LISTEN`。`lsof -ti:8080` 会匹配对端为 8080 的出站连接，据此杀进程会误杀无关程序（上个会话已误杀过一次 ToDesk） |

## 3. 尚未实现（按依赖顺序）

### 3.1 下一步：`frontend/` 与 `admin/`

后端 Entry API 已经完整：改 / 删 / 状态迁移 / Admin 读取于 2026-08-25 完成（2D，见第 8 节），`categories` 与 `unlisted` 于同日完成（2E，见第 9 节）。

下一步是 `frontend/`（Astro）与 `admin/`，两者都还没有目录。**接口契约见 `docs/api.md`**，那份是按当前实现逐条核对过的。`architecture.md` 写的是设计意图与理由，两份的路径已于 2026-08-25 统一为 `/api/v1/*`，但只有 `api.md` 对着运行中的服务核对过字段与状态码，冲突以它为准。

后端还剩的三件事都不阻塞前端，各自在 3.2 到 3.4：session 绝对过期、Argon2id 参数、trusted proxy。

`tags`、`media`、`archives`、`site` 四组接口尚未开始，见 3.5。`tags` 会落在 `internal/taxonomy` 里 —— 那个包从一开始就是为「categories 今天、tags 以后」写的。

### 3.2 待办：绝对过期

当前只有 7 天滑动过期，没有绝对上限。一个每天都在用的 session 可以无限续下去。这是当时确认的行为，但值得记下来：如果 token 泄露且攻击者保持活跃，这个 session 永不自动失效。

补救方式是 `sessions` 表加 `absolute_expires_at`，登录时定死，续期不动它。需要新 migration。

### 3.3 待办：Argon2id 参数

实测 **212ms/次**，参数 m=64MiB / t=3 / p=2。

含义：登录接口每请求占 212ms CPU。单 IP 在 5 次/分钟限流下最多消耗 1 秒 CPU/分钟，可接受；但分布式来源下这是放大器——攻击者发一个 HTTP 包，服务端花 212ms。

OWASP 推荐 m=46MiB / t=1 / p=1，约 50ms。`t=3` 是主要成本来源，降到 `t=1` 约 70ms，仍在推荐线上。个人站登录频率极低，倾向保持不变。**未决。**

### 3.4 待办：`trusted proxy` 与真实客户端 IP

**这一项现在有了实际后果，不再只是理论问题。** 限流已上线，而它按 `c.ClientIP()` 分桶。

`router.go` 目前 `SetTrustedProxies(nil)`，所以 `c.ClientIP()` 返回 socket 自身地址。本机直连时它是真实来源，正确。一旦前面放了 Caddy，所有请求的 `ClientIP` 都变成代理那一个地址，全部落进同一个桶：**限流从「每 IP 5 次/分钟」退化为「整站 5 次/分钟」，任何一个人打满额度就把站主一起锁在门外。**

部署前必须把真实代理地址填进 `SetTrustedProxies`，让 `ClientIP()` 读 `X-Forwarded-For`。这一点已写进 `middleware/ratelimit.go` 中 `Middleware` 的注释，就在读 `ClientIP` 的那几行旁边。

注意顺序：先配 `SetTrustedProxies` 再上代理。反过来做，中间那段时间限流是全局的。

### 3.5 后续阶段

Media 与对象存储（OSS）、Tag / Category、Markdown 渲染（前端负责，后端只存源文本）、Public API 与 Admin API 的分离、SEO 相关的服务端渲染、Docker、CI/CD。均未开始。

## 4. 悬而未决的产品问题

1. **编辑器选型** — CodeMirror 6 / Milkdown / 朴素 textarea。做 admin 时才需要定。
2. **服务器位置** — 影响延迟与备案。部署前定。

原第 3 条「第一批 `type` 取值」已定：六值一次定齐，见第 7 节。

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
```

`internal/entry` 与 `internal/entryhttp` 的拆分理由与 auth 那对相同，见下面的偏离说明。三个适配层之间**没有依赖**：`entryhttp` 自己声明 `AuthorResolver` 与 `CategoryResolver` 两个函数类型，由 router 分别接到 `authhttp.Authenticated` 和 `taxonomy.Service.ResolveSlug`，所以谁都不用 import 谁。

`taxonomy` 没有 Clock，`entry` 有。这不是遗漏：分类的 `created_at` 来自列默认值、`updated_at` 来自触发器，这个包里没有任何东西需要盖时间戳。文章的 `published_at` 才需要。

`internal/media` 与 `internal/storage` **尚未创建**，连空目录都没有。

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

本阶段有一个新 migration：`000004`，加 `categories` 表、`entries.category_id`、以及此前推迟的 `idx_entries_category`。因此现在是 **6 个部分索引 + 1 个 GIN + 1 个部分唯一索引**，与 architecture.md 一致，3.1 里那条「差一个索引」的记录已经作废。

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
