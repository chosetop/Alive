# 阶段方案：users + auth

> 待确认。本文档不含实现，确认后才动代码。

## 1. 当前项目状态（已核对，非记忆）

```
backend/                        1976 行，build / vet / test 全绿
├── cmd/server/main.go          配置加载 → 依赖组装 → 优雅关停
├── cmd/cli/main.go             只有 config:check 一个命令
├── internal/
│   ├── config/                 环境变量，零外部依赖，已有 6 组校验测试
│   ├── apperr/                 Error{Code, Message, Status, Fields, cause}
│   │                           已有 Unauthorized / Forbidden / Conflict 构造器
│   ├── httpx/                  data/error 信封 + request id context
│   ├── postgres/               pgxpool 封装，New / Ping / Stats / Close
│   ├── middleware/             RequestID / Logger / Recovery / CORS
│   ├── router/                 Dependencies{Config, Logger, Pool}
│   └── health/                 /health、/health/ready
├── migrations/000001_*.sql     citext 扩展 + set_updated_at() 触发函数
└── sql/queries/                空
```

环境事实：

| 项 | 状态 |
|---|---|
| PostgreSQL | 18.6 已运行（brew services），但 `alive` 角色和库都不存在 |
| `citext` | 可用 |
| `psql` | 已装 |
| `sqlc` | **未安装** |
| `golang-migrate` | **未安装** |
| Docker daemon | 未运行 |
| `golang.org/x/crypto` | 已在 go.mod，目前是 indirect |

已有的可复用件，方案会直接用而不重造：

- `apperr.Unauthorized()` 已存在，401 不需要新类型。
- `httpx.Error()` 已负责把 `*apperr.Error` 转成 error 信封并带上 request_id。
- `set_updated_at()` 触发函数已在 000001 里，users 表直接挂触发器即可。
- `postgres.Pool` 已封装好，repository 直接接收它。

## 2. 三个冲突，需要你裁决

### 冲突 A：URL 前缀

你要的是 `POST /api/auth/login`。但现状与已定架构是：

- `router.go:69` 已建好 `/api/v1` 组（当前为空）。
- 架构文档第 4 节定的是 `/api/v1/entries`。

如果 auth 挂在 `/api/auth`，而内容挂在 `/api/v1/entries`，会出现两个后果：

1. **URL 不一致。** 同一个 API 有两套前缀规则，以后每加一个领域都要先想「这个该带 v1 吗」。
2. **Cookie 的 `Path` 被迫放宽。** session Cookie 要同时被 `/api/auth/logout` 和 `/api/v1/entries` 带上，`Path` 只能设成 `/api`。设成 `/api/v1` 就覆盖不到 `/api/auth`。`Path` 越窄越好，它决定浏览器在哪些请求上附带这个凭证。

我的建议：**`POST /api/v1/auth/login`**，Cookie `Path=/api/v1`。

代价是你原话里的 URL 变了。要是你坚持 `/api/auth`，方案照做，Cookie `Path` 用 `/api`，我会在文档里记下这个取舍。

### 冲突 B：`role` 字段 vs 决策 5「不建 RBAC」

决策 5 的原文是「不建角色权限体系（RBAC），鉴权中间件只判断是否为已认证站主」。你现在要求 users 表含 `role`。

这两件事可以同时成立，但必须说清界线：

- **建 `role` 列**，`VARCHAR(16) NOT NULL DEFAULT 'owner'`。
- **不写任何读它的鉴权代码。** 中间件只判断「session 有效」，不看 role。
- 它现在的作用是给未来留一个已经存在的列，避免以后加列时要回填数据。

我要明确一句：**加了列却零代码读它，这是唯一能同时满足你这次要求和决策 5 的做法。** 一旦开始写 `if role == "admin"`，就是在建 RBAC 了，那要重新对齐决策 5。

### 冲突 C：sqlc 未安装，本阶段是否引入

技术选型定的是 sqlc。但：

- sqlc 未安装，需要 `brew install sqlc`（属于装工具，不是装项目依赖）。
- 本阶段的 SQL 只有 6 条查询，全是单表操作。
- sqlc 生成的代码要提交进仓库。

两个选项：

| | 现在就上 sqlc | 本阶段手写 pgx |
|---|---|---|
| 一致性 | 与技术选型一致，entries 阶段无缝 | entries 阶段要把 auth 的 SQL 迁一遍 |
| 前置动作 | 装 sqlc | 无 |
| 6 条查询的代码量 | 生成，但要配 schema 路径 | 约 120 行手写 |
| 类型安全 | 编译期由生成代码保证 | 靠 `Scan` 参数顺序，人工保证 |

我的建议：**现在就上 sqlc。** 理由是 entries 阶段的查询远比这里复杂（JSONB、部分索引、分页），届时 sqlc 是刚需；在只有 6 条简单查询的时候把它跑通，比在复杂查询上第一次踩它的坑要好。

## 3. 数据库设计

### migration 000002

按你的要求，`users` 与 `sessions` 同在 `000002_create_users`。

```sql
CREATE TABLE users (
    id            BIGSERIAL    PRIMARY KEY,
    username      CITEXT       NOT NULL,
    password_hash TEXT         NOT NULL,
    role          VARCHAR(16)  NOT NULL DEFAULT 'owner',
    display_name  VARCHAR(64),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT users_username_key UNIQUE (username),
    CONSTRAINT users_username_len CHECK (length(username) BETWEEN 3 AND 64)
);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

字段说明：

- `username` 用 `CITEXT` 而非 `VARCHAR`。000001 已装这个扩展，正是为此。效果：`Alice` 和 `alice` 是同一个账号，登录不因大小写失败，且唯一约束自动是大小写无关的。用 `VARCHAR` 则要在应用层记得每次 `lower()`，漏一次就出现两个「同名」账号。
- `password_hash` 用 `TEXT` 不设长度。Argon2id 的编码串约 100 字符，但参数调整后会变长，写死长度是给未来埋雷。
- `role` 见冲突 B。
- `display_name` 可空。架构文档 2.2 的 users 表有这个字段，前台展示名用，登录不依赖它。

```sql
CREATE TABLE sessions (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    token_hash BYTEA       NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- 观测用，不参与鉴权判断
    user_agent TEXT,
    ip         INET,

    CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT sessions_token_hash_key UNIQUE (token_hash)
);

-- 鉴权主查询：按 token_hash 精确查找
CREATE INDEX sessions_token_hash_idx ON sessions (token_hash);

-- 清理过期 session，以及「踢掉某用户所有会话」
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
CREATE INDEX sessions_user_id_idx    ON sessions (user_id);
```

几处需要解释：

- **`token_hash` 用 `BYTEA` 而非 `TEXT`。** SHA-256 输出是 32 字节二进制，存 `BYTEA` 是 32 字节；转成 hex 存 `TEXT` 是 64 字节，且每次比较要多做一次编码。没有理由把二进制当文本存。
- **`ON DELETE CASCADE`。** 删用户就删他的全部 session。这里用 CASCADE 而不是 RESTRICT，因为 session 是用户的附属物，不是独立数据。反过来 `entries.author_id` 将来必须用 RESTRICT，那是不能被静默删除的内容。
- **`sessions` 表没有软删除。** 登出就是 `DELETE`。session 是凭证不是内容，保留一条「已失效的凭证」没有任何用途，只增加查询时必须记得过滤的条件。这与 entries 用 `deleted_at` 软删除是两种不同性质的数据，不应统一。
- **`ip` 用 `INET` 类型。** Postgres 原生类型，能存 IPv4 和 IPv6，且能做网段查询。存 `VARCHAR` 就只能当字符串比较。
- **`user_agent` 和 `ip` 只用于观测**，比如你想知道「这个 session 是从哪登的」。**鉴权时绝不校验它们。** 校验 IP 会让移动网络切换基站就掉登录状态，校验 UA 会让浏览器自动更新就掉登录状态，二者带来的安全收益远低于误伤。

### sqlc 查询清单（6 条）

```
sql/queries/auth.sql
  CreateUser        插入用户，返回 id 与时间戳
  GetUserByUsername 登录用，取 id/username/password_hash
  GetUserByID       受保护路由用（若走 join 则可省）
  CreateSession     插入 session
  GetSessionByHash  鉴权主查询，join users 一次取回用户
  DeleteSession     登出
  DeleteExpiredSessions  清理，供 CLI 调用
```

`GetSessionByHash` 用 join 一次取回 session 与 user，鉴权路径上只查一次库。分两次查会让每个受保护请求多一次往返。

## 4. 文件清单

> 实施记录（2026-08-24）：本节已按实际落地情况更新。与最初计划的差异在小节末尾说明。

### 新增

```
migrations/
  000002_create_users.up.sql        users + sessions + 索引 + 触发器          已完成
  000002_create_users.down.sql      逆序 DROP                                已完成

sql/queries/
  auth.sql                          10 条 sqlc 查询                          已完成

internal/postgres/sqlcgen/          sqlc 生成，提交进仓库                     已完成
  db.go  models.go  querier.go  auth.sql.go

internal/auth/                      领域包，不依赖 Gin
  model.go          领域类型 + sentinel error                                已完成
  password.go       Argon2id 哈希与校验                                      已完成
  token.go          session token 生成与 SHA-256                             已完成
  repository.go     包装 sqlcgen，翻译驱动错误                                已完成
  service.go        Login / Authenticate / Logout / CreateUser               已完成
  authtest/store.go 内存 Store，供两个测试包共用                              已完成

internal/authhttp/                  HTTP 适配层，唯一接触 Gin 的地方
  cookie.go         Cookie 属性集中一处                                      已完成
  dto.go            loginRequest / userResponse                             已完成
  handler.go        解析请求、设 Cookie、调 service                           已完成
  middleware.go     鉴权中间件 + context 存取                                已完成

internal/middleware/
  ratelimit.go      进程内 IP 限流                                           已完成

测试
  internal/auth/        model_test.go  password_test.go  token_test.go
                        service_test.go  repository_test.go
                        repository_session_test.go
  internal/authhttp/    cookie_test.go  handler_test.go
  internal/middleware/  ratelimit_test.go                                   已完成
```

#### 与原计划的两处差异

**一、HTTP 适配层独立成 `internal/authhttp`，不在 `internal/auth` 内。**

Go 的依赖是包级的，不是文件级的。若 `handler.go` 位于 `internal/auth` 并 import Gin，整个 `internal/auth` 包就依赖 Gin，`cmd/cli` 哪怕只用 `user:create` 也会链接进整个 Web 框架；而「auth 核心不依赖 Gin」这条要求会退化成只靠 code review 维持的约定。

拆包后该边界由编译器保证，可验证：

```
go list -deps ./cmd/cli       | grep -c gin-gonic   →  0
go list -deps ./internal/auth | grep -c gin-gonic   →  0
```

`internal/health` 仍将 handler 放在包内，因为 health 没有非 HTTP 的使用者；auth 有。

**二、查询数量 6 → 10。** 原文说「6 条查询」，实际需求是 10 条。以需求为准，未为了维持数字而删减必要查询。

### 修改

```
internal/router/router.go     加 AuthService；注册 auth 路由；启动期缺失依赖 panic   已完成
internal/config/config.go     加 SessionConfig                                     已完成
internal/config/config.go     加 RateLimitConfig                                   已完成
internal/router/router.go     限流中间件挂到登录路由                              已完成
cmd/cli/main.go               config:check 打印限流配置                           已完成
.env.example / .env           新增限流配置项                                      已完成
internal/apperr/apperr.go     加 INVALID_CREDENTIALS 与 RATE_LIMITED               已完成
internal/auth/model.go        Authenticated 加 Renewed 字段                        已完成（见下）
cmd/server/main.go            组装 auth repository → service                       已完成
cmd/cli/main.go               config:check 打印 session 配置                       已完成
cmd/cli/main.go               加 user:create 与 session:prune                      未开始
.env.example                  新增 session 配置项                                  已完成
Makefile                      加 user-create 目标                                  未开始
README.md                     auth 相关的验证步骤                                  未开始
docs/architecture.md          5.4 节改为「已定：服务端 session」，删掉方案 B         未开始
```

#### 计划外的一处改动：`Authenticated.Renewed`

滑动过期要求 session 续期后 Cookie 的 `Max-Age` 也跟着走，否则数据库里的有效期向前滑动而浏览器仍按原计划过期，站主会在使用中途被登出。

问题是适配层无法从 `ExpiresAt` 推断「这次请求是否续期了」——刚续期的 session 和刚创建的 session 在时间上完全一样。所以由 Service 直接报告，且仅在写入**成功**时置 true：一个承诺了数据库里并不存在的有效期的 Cookie，会让浏览器在服务端过期之后继续发送 token。

### 不新增

`internal/entry/`、`internal/taxonomy/`、`internal/media/`、`internal/storage/` 一个都不建，连空目录都不建。

## 5. Session 生命周期

```
┌─ 登录 ────────────────────────────────────────────────────┐
│ 1. 限流中间件按 IP 检查，超限 → 429                        │
│ 2. 取 users.username（CITEXT，大小写无关）                 │
│ 3. Argon2id 校验密码                                      │
│    用户不存在时也跑一次假校验，避免时间差泄露用户是否存在    │
│ 4. crypto/rand 生成 32 字节 token                         │
│ 5. SHA-256(token) 存库，明文只出现在 Set-Cookie 里         │
│ 6. expires_at = now + 7d                                  │
└───────────────────────────────────────────────────────────┘
                          │
┌─ 使用 ────────────────────────────────────────────────────┐
│ 每个受保护请求：                                           │
│   读 Cookie → SHA-256 → 查库（join users）                │
│   查不到或 expires_at 已过 → 401                          │
│                                                            │
│ 滑动续期：剩余寿命 < 50% 时才 UPDATE expires_at            │
│   为什么不是每次都续：每个请求一次写库，读多写少的接口       │
│   会被这一条拖成写密集型                                    │
└───────────────────────────────────────────────────────────┘
                          │
┌─ 终止 ────────────────────────────────────────────────────┐
│ 登出       DELETE 该行 + 下发过期 Cookie                   │
│ 自然过期   下次查询时发现已过期 → 401，并顺手 DELETE       │
│ 删用户     ON DELETE CASCADE 连带删除                      │
│ 批量清理   CLI session:prune 删所有过期行                  │
└───────────────────────────────────────────────────────────┘
```

绝对上限 7 天，滑动续期不突破它：`expires_at` 每次续期都是 `now + 7d`，所以一个一直在用的 session 可以长期有效。如果你要「无论如何 30 天必须重新登录」，需要另加 `absolute_expires_at` 字段。**这一条我按不加处理，你要的话说一声。**

## 6. 三条调用链

### 登录

```
POST /api/v1/auth/login
  │
  ├─ RequestID → Logger → Recovery → CORS        已有
  ├─ RateLimit(5/min per IP)                      新增，仅挂此路由
  │    超限 → apperr 429 → httpx.Error
  │
  └─ auth.Handler.Login                           Gin 适配层
       ├─ c.ShouldBindJSON(&loginRequest)
       │    格式错 → apperr.InvalidInput → 400
       │
       ├─ auth.Service.Login(ctx, username, password, meta)
       │    │                                     纯 Go，无 gin 依赖
       │    ├─ Repository.GetUserByUsername
       │    │    不存在 → 仍跑一次假 Argon2id 校验 → ErrInvalidCredentials
       │    ├─ password.Verify(hash, password)
       │    │    不匹配 → ErrInvalidCredentials
       │    ├─ token.New()          32 字节随机
       │    ├─ Repository.CreateSession(hash, expiresAt)
       │    └─ 返回 (User, plaintextToken, expiresAt)
       │
       ├─ cookie.Set(c, token, expiresAt)         唯一接触 Set-Cookie 的地方
       └─ httpx.OK(c, userResponse)               不含 token，token 只在 Cookie
```

失败一律返回 `401 INVALID_CREDENTIALS`，不区分「用户不存在」和「密码错误」。区分了就等于送给攻击者一个用户名枚举接口。

### 登出

```
POST /api/v1/auth/logout
  │
  ├─ 基础中间件
  ├─ auth.RequireSession                          需要已登录
  └─ auth.Handler.Logout
       ├─ auth.Service.Logout(ctx, tokenHash)     DELETE
       └─ cookie.Clear(c)                         MaxAge=-1
       └─ 204 No Content
```

登出对「Cookie 里的 token 已经无效」也返回成功。幂等，因为客户端的目标是「变成未登录状态」，而它已经达成了。

### 鉴权中间件

```
任意受保护路由
  │
  └─ auth.RequireSession(service)                 Gin 适配层
       ├─ c.Cookie(name)
       │    无 Cookie → apperr.Unauthorized → 401
       │
       ├─ auth.Service.Authenticate(ctx, token)   纯 Go
       │    ├─ SHA-256(token)
       │    ├─ Repository.GetSessionByHash        join users
       │    │    未命中 → ErrSessionInvalid
       │    ├─ expires_at 已过 → 删除 + ErrSessionExpired
       │    └─ 剩余 < 50% → 续期
       │
       ├─ 失败 → cookie.Clear + 401
       │    清 Cookie 是为了让浏览器别再带一个已知无效的凭证
       │
       └─ 成功 → 把 User 放进 request context，c.Next()
```

`internal/middleware` 不放这个中间件，它放在 `internal/auth/middleware.go`。原因：`internal/middleware` 是通用 HTTP 关注点，不该 import 业务领域包；而这个中间件必须持有 `auth.Service`。按领域切分的目录结构下，它属于 auth。

## 7. 密码哈希：Argon2id

技术选型表里写的是 bcrypt。**我建议改成 Argon2id**，理由如下。

bcrypt 的三个具体问题：

1. **72 字节截断。** 超过 72 字节的密码，后面的部分被静默忽略。用密码管理器生成的长密码会受影响，且失败方式是「安全性变低但一切正常」，不会报错。
2. **只有 CPU 成本，没有内存成本。** GPU 和 ASIC 破解 bcrypt 的效率远高于 CPU，因为它对内存需求很小，可以大规模并行。
3. Argon2 是 2015 年密码哈希竞赛（PHC）的获胜方案，也是 OWASP 当前的首选推荐。

Argon2id 的参数，按 OWASP 建议取：

```
memory      64 MiB
iterations  3
parallelism 2
salt        16 字节随机
keyLen      32 字节
```

存储格式用 PHC 标准串，参数编码在哈希里：

```
$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
```

这样以后调参不影响老密码：校验时从串里读出当时的参数。

**代价，说实话：** bcrypt 的 `golang.org/x/crypto/bcrypt` 自带这套编码解码，两行就能用完。Argon2id 的 `golang.org/x/crypto/argon2` 只给纯 KDF 函数，PHC 串的编码和解析要自己写，约 40 行。这 40 行是这个选择的全部额外成本。

依赖上不新增模块：`golang.org/x/crypto` 已在 go.mod（目前是 indirect，会变成 direct）。

校验必须用 `crypto/subtle.ConstantTimeCompare`，不能用 `==`。字节比较提前返回会泄露前缀匹配长度。

**这一条如果你更想要 bcrypt 的简单，说一声，我按 bcrypt 做，代价是上面三点。**

## 8. Cookie 安全属性

```go
http.Cookie{
    Name:     "alive_session",
    Value:    token,        // 43 字符 base64url，明文只在这里出现
    Path:     "/api/v1",    // 取决于冲突 A 的裁决
    HttpOnly: true,
    Secure:   cfg.Session.CookieSecure,
    SameSite: http.SameSiteStrictMode,
    Expires:  expiresAt,
    // 不设 Domain
}
```

逐条说明：

| 属性 | 值 | 为什么 |
|---|---|---|
| `HttpOnly` | true | JS 读不到。有了它，XSS 也偷不走 session token |
| `Secure` | 生产 true | 只走 HTTPS。开发环境是 http://localhost，设 true 会让 Cookie 根本不下发，所以按环境取值 |
| `SameSite` | Strict | 跨站请求完全不带此 Cookie，CSRF 面基本关闭。这是同域 `/admin` 决策换来的，子域方案只能用 Lax |
| `Path` | `/api/v1` | 只在 API 请求上附带。访问 `/` 或 `/admin` 的静态资源不带凭证 |
| `Domain` | **不设** | 不设则只对当前主机生效。设 `.域名` 会把凭证暴露给所有子域，包括你以后可能加的任何一个 |
| `Expires` | = session expires_at | 与服务端一致。浏览器到期自动丢弃，省一次注定失败的请求 |

`Secure` 按环境取值这件事有个陷阱要注意：如果生产忘了设 `APP_ENV=production`，Cookie 会以非 Secure 下发，凭证就可能走明文。我会让 `config.validate()` 强制：**`APP_ENV=production` 时 `SESSION_COOKIE_SECURE` 必须为 true**，配成 false 直接启动失败。

因为 `SameSite=Strict` 已经关闭了跨站请求，**本阶段不引入 CSRF token**。加了是重复防护，且要在 admin 端配套一套取 token 的逻辑。

## 9. 限流

进程内，不引入 Redis。

```
固定窗口计数器：
  key    客户端 IP
  window 1 分钟
  limit  5 次

超限响应：
  429 + Retry-After: <剩余秒数>
  body: {"error":{"code":"RATE_LIMITED",...}}
```

> 实施记录：错误码定为 `RATE_LIMITED`，不是本节原文的 `TOO_MANY_REQUESTS`。`apperr` 里对应 `CodeRateLimited` 与 `RateLimited()` 构造器。原因是状态码本身已经是 429，错误码再重复一遍「次数太多」没有新增信息，而 `RATE_LIMITED` 说的是「你被限流器拦下了」，与状态码是两件不同的事。

实现要点：

- `map[string]*counter` + `sync.Mutex`。这个量级不需要分片锁。
- **必须有清理机制**，否则每个来访 IP 留一个永不释放的条目，这是内存泄漏。用惰性清理：每次访问时顺手删掉已过期的条目，不起后台 goroutine。少一个需要管生命周期的东西。
- `apperr` 需新增 429 构造器，目前没有。**已完成**，落地为 `CodeRateLimited` / `RateLimited()`。

实施时补充的两条，原方案未写明：

- **被拒的请求不计数。** 若把拒绝也计入，`resetAt` 会被每次重试推后，客户端越重试锁得越久，「固定窗口」就名不副实了。已有测试覆盖这条。
- **限流器在启动时构造一次，不在请求路径里构造。** 后者会让计数每次归零，限流永不触发，而且这个错误在生产环境完全看不出来——日志、状态码、响应体全都正常。`router_test.go` 里有一条测试专门盯这个。

**一个必须指出的问题：** `router.go:55` 现在是 `SetTrustedProxies(nil)`，所以 `c.ClientIP()` 返回的是直连的 socket 地址。生产环境 Caddy 在前面时，那个地址永远是 `127.0.0.1`，**所有请求会落进同一个桶，限流实际上退化成「整站每分钟 5 次登录」**。

这不是现在要解决的问题，但上线前必须处理：把 Caddy 的地址配进 `SetTrustedProxies`，让 `ClientIP()` 读 `X-Forwarded-For`。我会在代码注释和 README 里写明这一点，避免它成为一个没人记得的隐患。

固定窗口的已知弱点：窗口边界允许两倍突发（第 59 秒 5 次 + 第 61 秒 5 次）。对「阻止暴力破解」这个目标，这个弱点无关紧要。滑动窗口要存每次请求的时间戳，内存和复杂度都更高，不值得。

## 10. curl 验证方案

前置：建库建角色，跑 migration，建账号。

```bash
# 建角色与库（psql 已装，PostgreSQL 18.6 已在运行）
createuser -s alive
createdb -O alive alive
psql -d alive -c "ALTER USER alive WITH PASSWORD 'alive';"

# 装工具
brew install golang-migrate sqlc

# 迁移
make migrate-up
make migrate-version          # 期望 2

# 建站主账号
go run ./cmd/cli user:create --username owner
# 交互式读密码，不回显，不进 shell history
```

验证序列：

```bash
# 1. 未登录访问受保护路由 → 401
curl -i http://127.0.0.1:8080/api/v1/me
# HTTP/1.1 401
# {"error":{"code":"UNAUTHORIZED","message":"authentication required",...}}

# 2. 密码错误 → 401，且信息不透露用户是否存在
curl -i -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"owner","password":"wrong"}'
# HTTP/1.1 401  INVALID_CREDENTIALS

# 3. 不存在的用户 → 完全相同的响应
curl -i -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"nobody","password":"wrong"}'
# HTTP/1.1 401  INVALID_CREDENTIALS   ← 与上一条逐字相同

# 4. 登录成功 → 200 + Set-Cookie
curl -i -c /tmp/alive-cookies.txt -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"owner","password":"<真实密码>"}'
# Set-Cookie: alive_session=...; Path=/api/v1; Expires=...; HttpOnly; SameSite=Strict
# {"data":{"id":1,"username":"owner",...}}      ← 响应体里没有 token

# 5. 检查 Cookie 属性
cat /tmp/alive-cookies.txt
# 应看到 HttpOnly 标记；开发环境 Secure 为 false

# 6. 带 Cookie 访问受保护路由 → 200
curl -i -b /tmp/alive-cookies.txt http://127.0.0.1:8080/api/v1/me
# {"data":{"id":1,"username":"owner","role":"owner",...}}

# 7. 库里只有 hash，没有明文 token
psql -d alive -c "SELECT id, user_id, length(token_hash) AS hash_bytes, expires_at FROM sessions;"
#  id | user_id | hash_bytes |        expires_at
#   1 |       1 |         32 | 2026-08-31 ...
# 32 字节 = SHA-256。表里没有任何列存明文

# 8. 密码不是明文
psql -d alive -c "SELECT username, left(password_hash, 40) FROM users;"
# owner | $argon2id$v=19$m=65536,t=3,p=2$...

# 9. 登出 → 204，且库里的行消失
curl -i -b /tmp/alive-cookies.txt -X POST http://127.0.0.1:8080/api/v1/auth/logout
# HTTP/1.1 204
psql -d alive -c "SELECT count(*) FROM sessions;"    # 0

# 10. 登出后旧 Cookie 失效 → 401
curl -i -b /tmp/alive-cookies.txt http://127.0.0.1:8080/api/v1/me
# HTTP/1.1 401

# 11. 限流：连打 7 次，前 5 次 401，后 2 次 429
for i in $(seq 1 7); do
  printf '%d: ' "$i"
  curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"owner","password":"wrong"}'
done
# 1: 401
# ...
# 5: 401
# 6: 429      ← Retry-After 头应存在
# 7: 429

# 12. 过期 session 立即失效（手工把 expires_at 改到过去）
psql -d alive -c "UPDATE sessions SET expires_at = now() - interval '1 hour';"
curl -i -b /tmp/alive-cookies.txt http://127.0.0.1:8080/api/v1/me    # 401
```

自动化测试覆盖（不需要数据库的部分）：

- `password_test.go` — 哈希可校验、错密码失败、PHC 串可往返解析、相同密码两次哈希不同（salt 随机）
- `token_test.go` — 长度、字符集、SHA-256 稳定、不重复
- `service_test.go` — 用假 repository 覆盖：错密码、用户不存在、session 过期、续期阈值
- `middleware_test.go` — 无 Cookie 401、坏 Cookie 401、有效 Cookie 放行并注入 user
- `ratelimit_test.go` — 第 6 次 429、窗口过后恢复、不同 IP 互不影响、过期条目被清理

需要数据库的 repository 测试，本阶段不写。它需要一套测试库的建立与清理机制，那是独立的一件事，建议在 entries 阶段连同 sqlc 的复杂查询一起做。

## 待你确认

| # | 问题 | 我的建议 |
|---|---|---|
| A | URL 用 `/api/v1/auth/login` 还是你原话的 `/api/auth/login`？ | `/api/v1/auth/login`，Cookie `Path=/api/v1` |
| B | `role` 列建但零代码读它，可以吗？ | 可以，这是同时满足你要求与决策 5 的唯一做法 |
| C | 本阶段就上 sqlc，还是手写 pgx？ | 上 sqlc，需 `brew install sqlc golang-migrate` |
| D | 密码哈希用 Argon2id 还是表里原定的 bcrypt？ | Argon2id，代价是自己写 40 行 PHC 编解码 |
| E | session 要不要绝对上限（如 30 天必须重登）？ | 不要，7 天滑动续期够用 |
| F | 受保护测试路由用 `GET /api/v1/me`？ | 是，它同时是 admin 判断登录状态的真实接口 |
