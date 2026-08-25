# Alive 后端接口文档

对应实现：`backend/internal/router/router.go`（全部 URL 的唯一登记处）。

本文档描述**已实现并通过实机验证**的接口。`tags`、`media`、`archives`、`site` 四组尚未开始，不在此列。`architecture.md` 里出现但这里没有的接口，都属于还没写。

本文档里的每个字段名、默认值、状态码都对着真实服务核对过（2026-08-25，`alive_test` + 8099 端口），不是从代码推的。写「已实测」的地方是这一轮抓到过我自己写错的地方。

与 `architecture.md` 的分工：那份讲设计意图与理由，这份讲**当前实现**的字段、状态码与边界。两份的路径已统一为 `/api/v1/*`，但只有这份是对着运行中的服务核对过的，冲突时以这份为准。

基础路径 `/api/v1`。探针在根路径，不带版本前缀：探针 URL 不应该因为 API 升版而改变。

---

## 1. 通用约定

### 1.1 响应信封

每个响应体都是一个 JSON 对象，顶层**有且仅有一个**主键：成功是 `data`，失败是 `error`。客户端因此不看状态码也能分辨两者。

成功：

```jsonc
{ "data": { } }
```

分页的成功响应多一个 `meta`：

```jsonc
{
  "data": [ ],
  "meta": { "page": 1, "page_size": 20, "total": 37 }
}
```

`meta.total` 统计的是**当前筛选条件下**的总数，不是表里的总行数。所以客户端不会翻向永远取不到的内容。

失败：

```jsonc
{
  "error": {
    "code": "INVALID_INPUT",
    "message": "this entry cannot be saved as submitted",
    "fields": { "slug": "must be lowercase letters and digits joined by single hyphens" },
    "request_id": "7edcebd2c299a9161e5190efb0adcea9"
  }
}
```

- `code` 机器可读，客户端可以分支判断。发布后不会改。
- `message` 给人看，可能会改，**不要拿它做判断**。
- `fields` 只在校验失败时出现，键是字段名。
- `request_id` 报错时报这一个值就能找到对应日志行。

底层驱动的错误信息**永远不进响应体**，只进日志。数据库错误原文会把表名和列名交给提问的人。

### 1.2 错误码与状态码

| code | HTTP | 含义 |
|---|---|---|
| `INVALID_INPUT` | 400 / 405 | 请求本身有问题：JSON 不合法、缺必填字段、字段值不合规。**405 也用这个 code**，见下 |
| `INVALID_CREDENTIALS` | 401 | 登录被拒：用户名或密码错 |
| `UNAUTHORIZED` | 401 | 请求没带可用会话，或会话已失效 |
| `FORBIDDEN` | 403 | 身份有效但无权做这件事 |
| `NOT_FOUND` | 404 | 目标不存在，或存在但当前身份不可见 |
| `CONFLICT` | 409 | 请求合法，但与现有数据冲突（目前只有 slug 重复） |
| `RATE_LIMITED` | 429 | 尝试次数过多，带 `Retry-After` 头 |
| `INTERNAL` | 500 | 服务端故障 |
| `UNAVAILABLE` | 503 | 依赖不可用（就绪探针用） |

`INVALID_CREDENTIALS` 与 `UNAUTHORIZED` 都是 401，但分开是有意的：前者是「密码错了」，后者是「请重新登录」，后台要对这两种情况显示不同的东西。

**404 有两种来源，客户端分不出也不需要分**：内容真的不存在，和内容存在但你不能看（草稿、private、已软删）。返回 403 会等于确认「这个 slug 上有东西」，那正是枚举草稿标题的入口。

### 1.3 分页

| 参数 | 默认 | 上限 | 说明 |
|---|---|---|---|
| `page` | 1 | 无 | 从 1 开始 |
| `page_size` | 20 | 50 | 超过上限按上限处理，不报错 |

越界值**被夹紧而不是拒绝**。请求第 4 页而集合已经缩短到 2 页，返回空页——内容被撤下发布时集合会变小，一秒前还存在的页码不是客户端的错。

已实测：`page_size=100000` 得到 `page_size: 50`；`page=9999` 得到 `data: []` 且 `meta.page: 9999`。

### 1.4 时间

全部是带时区的 RFC 3339，例如 `2026-08-25T12:10:48.808389+08:00`。

可空的时间字段用 `null` 表示「没有」，不是零值时间：`happened_at` 可以没有（不是所有内容都对应一个日期），`published_at` 在从未发布过的内容上不存在。

### 1.5 认证

会话是 Cookie，不是 Bearer token。登录成功时服务端下发：

```
Set-Cookie: alive_session=<token>; Path=/api/v1; HttpOnly; Secure; SameSite=Strict; Max-Age=604800
```

- `HttpOnly`：`document.cookie` 读不到。被注入的脚本因此偷不走它。
- `SameSite=Strict`：跨站请求不带这个 Cookie。
- `Secure`：只走 HTTPS。**默认 `true`，与环境无关**；生产环境强制 `true`（配置里设成 false 会拒绝启动）。本地跑 `http://localhost` 时如果登录看起来「没生效」，就是这个：Secure cookie 不会在明文 HTTP 上发送，需要显式设 `SESSION_COOKIE_SECURE=false`。
- `Path=/api/v1`：凭证只发往 API。
- 有效期 7 天滑动续期。**目前没有绝对上限**：每天都在用的会话可以无限续下去。

响应体里**没有 token**。它只存在于 `Set-Cookie` 头里。数据库里存的是 `sha256(token)`，所以库里不存在可重放的凭证。

前端调用要点：`fetch` 必须带 `credentials: "include"`，且 CORS 的 `CORS_ALLOWED_ORIGINS` 要列出前端源（不能用 `*`，因为这个 API 允许携带凭证）。

### 1.6 寻址：什么时候用 id，什么时候用 slug

**后台按 `id` 寻址，前台按 `slug` 寻址。**

这不是不一致。读者靠 URL 认一篇文章，而编辑改的可能正是 slug 本身——用 slug 寻址会让「改标题」变成「换了一个资源」。

分类的写操作全部按 id，因为 slug 也是被编辑的对象之一。

### 1.7 PATCH 的字段约定

这一条适用于所有 `PATCH` 接口，**读之前不要动写接口**。

- **字段缺省** = 不改这个字段
- **`null`** = 也是不改。`encoding/json` 对显式 `null` 和缺键都留 `nil` 指针，两者分不开
- **空值（`""` / `0`）** = 清空这个字段

所以清空可空字段用空值，不是 `null`：

```jsonc
{ "summary": "" }        // 清空摘要
{ "category_id": 0 }     // 变成未分类
{ "happened_at": "0001-01-01T00:00:00Z" }  // 清空日期

{ "summary": "有值", "cover_url": null }   // 改 summary，cover_url 保持原样
```

`happened_at` 是这个约定唯一别扭的地方：`""` 是天然的空字符串，但零值时间戳不是天然的空日期。

**空 body（`{}`）返回 400**，不是 200。一个什么都不改的请求是客户端构造错了，返回 200 会把这个错误藏起来。

**推论：一个只含 `null` 的 body 也是 400。** `{"summary": null}` 单独发出去等于 `{}`——既然 `null` 读作「未提交」，那这个请求就没有提交任何字段。`null` 只有在**和至少一个真实字段同时出现**时才是「保持原样」（上面最后一个例子）。这一条实测过，内容和分类两边都是 400 `INVALID_INPUT` / `fields.body`。

---

## 2. 接口总览

| Method | Path | 认证 | 状态码 |
|---|---|---|---|
| POST | `/api/v1/auth/login` | 公开 | 200 / 400 / 401 / 429 |
| POST | `/api/v1/auth/logout` | 公开 | 204 |
| GET | `/api/v1/me` | 需登录 | 200 / 401 |
| GET | `/api/v1/entries` | 公开 | 200 / 400 / 404 |
| GET | `/api/v1/entries/:slug` | 公开 | 200 / 404 |
| POST | `/api/v1/entries` | 需登录 | 201 / 400 / 401 / 409 |
| PATCH | `/api/v1/entries/:id` | 需登录 | 200 / 400 / 401 / 404 / 409 |
| DELETE | `/api/v1/entries/:id` | 需登录 | 204 / 400 / 401 / 404 |
| POST | `/api/v1/entries/:id/publish` | 需登录 | 200 / 400 / 401 / 404 |
| POST | `/api/v1/entries/:id/unpublish` | 需登录 | 200 / 400 / 401 / 404 |
| POST | `/api/v1/entries/:id/archive` | 需登录 | 200 / 400 / 401 / 404 |
| GET | `/api/v1/admin/entries` | 需登录 | 200 / 400 / 401 |
| GET | `/api/v1/admin/entries/:id` | 需登录 | 200 / 400 / 401 / 404 |
| GET | `/api/v1/categories` | 公开 | 200 |
| POST | `/api/v1/categories` | 需登录 | 201 / 400 / 401 / 409 |
| PATCH | `/api/v1/categories/:id` | 需登录 | 200 / 400 / 401 / 404 / 409 |
| DELETE | `/api/v1/categories/:id` | 需登录 | 204 / 400 / 401 / 404 |
| GET | `/api/v1/admin/categories` | 需登录 | 200 / 401 |
| GET | `/api/v1/admin/categories/:id` | 需登录 | 200 / 400 / 401 / 404 |
| GET | `/health` | 公开 | 200 |
| GET | `/health/ready` | 公开 | 200 / 503 |

路径不存在返回 404 `NOT_FOUND`，方法不支持返回 **405**（不是 404）且 code 是 `INVALID_INPUT`。gin 默认把方法错误报成 404，那会把一个常见的客户端错误藏在误导性的状态码后面。

**所以 `INVALID_INPUT` 不能等同于 400。** 要区分这两种情况就看状态码，别只看 code。

---

## 3. 认证

### `POST /api/v1/auth/login`

公开，**有限流**。

```jsonc
{ "username": "owner", "password": "..." }
```

两个字段都必填。**没有长度校验**：比最短长度还短的密码不是「请求格式错」而是「密码错」，把两者区别对待等于告诉攻击者真密码不是多长。

成功 200，下发 `Set-Cookie`：

```jsonc
{ "data": { "id": 1, "username": "owner", "role": "owner", "display_name": "" } }
```

失败：

| 情况 | 状态 | code |
|---|---|---|
| 缺字段 / JSON 不合法 | 400 | `INVALID_INPUT` |
| 密码错 | 401 | `INVALID_CREDENTIALS` |
| 用户不存在 | 401 | `INVALID_CREDENTIALS` |
| 尝试过多 | 429 | `RATE_LIMITED`，带 `Retry-After` |

**「密码错」与「用户不存在」的响应逐字节相同**，且耗时相当（用户不存在时也走一次密码哈希）。否则响应差异或时间差异就是一个可以用来枚举用户名的信道。

限流是固定窗口计数器，只挂在这一个路由前，进程级共享。被拒的请求**不做密码哈希**（实测 54µs 对比正常失败 110ms），且不延长窗口。

### `POST /api/v1/auth/logout`

公开，**幂等**，永远 204。

不需要带有效会话：登出的目的是让凭证失效，对一个已经失效的凭证再调一次不是错误。重复调用同样 204。

响应清除 Cookie（`Max-Age=0`），同时删除库里的会话行。

### `GET /api/v1/me`

需登录。返回当前会话对应的账号，形状同 login 的 `data`。

会话无效或缺失返回 401 `UNAUTHORIZED`。

---

## 4. 内容（entries）

### 4.1 三个枚举

**`type`**（六值，数据库 CHECK 约束限定）：`journal` `book` `movie` `music` `travel` `photo`

目前只实现了 `journal` 的 `meta` 结构。加第七种类型需要一个 migration，这个成本是有意的：它强制新类型经过一次显式决策，而不是随手写一个字符串。

**`status`**：

| 值 | 含义 | 前台可见 |
|---|---|---|
| `draft` | 没写完 | 否 |
| `published` | 已发布 | 是 |
| `archived` | 写完了又撤下 | 否 |

`draft` 和 `archived` 是两件事：草稿是待办队列，归档不是。后台列表要区别对待。

**`visibility`**：

| 值 | 出现在列表 | 凭 slug 可达 |
|---|---|---|
| `public` | 是 | 是 |
| `unlisted` | **否** | **是** |
| `private` | 否 | 否 |

`unlisted` 是这三者里唯一不对称的：不出现在列表、计数、sitemap 里，但有链接就能打开。

**`unlisted` 不是访问控制。** slug 是人可读的，因此是可猜的。真正不能给陌生人看的内容用 `private`。

### 4.2 前台读取

#### `GET /api/v1/entries`

公开，分页。只返回 `status=published AND visibility=public AND deleted_at IS NULL`。

| 参数 | 说明 |
|---|---|
| `page` / `page_size` | 见 1.3 |
| `category` | 分类 **slug** |

`category` 传的是 slug，但过滤在 SQL 里用 id：请求先把 slug 解析成 id。

- **解析不到是 404，不是空列表。** `?category=nope`（打错的链接）和 `?category=空分类` 是两回事，客户端分不清就会把前者显示成「这个分类下暂无内容」
- **传空串（`?category=`）等同于没传。** 前端从表单状态拼查询串时，分类选「全部」发出的就是这个，对它报 400 会打断这个筛选器唯一要服务的场景
- 分类筛选**只会收窄可见范围，不会放宽**：加了 `category` 之后上面那三个条件一个都不少

响应（`data` 是数组，`meta` 见 1.1）：

```jsonc
{
  "type": "travel",
  "title": "京都的春天",
  "slug": "kyoto-spring",
  "summary": "在鸭川边坐了一整个下午",
  "cover_url": "https://cdn.example.com/a.webp",
  "meta": {},
  "word_count": 1200,
  "category": { "id": 3, "name": "旅行", "slug": "travel" },
  "happened_at": "2023-04-03T12:00:00Z",
  "published_at": "2026-08-25T12:10:48+08:00"
}
```

**不含 `content_md`**：列表页不渲染正文，返回它会让首屏响应膨胀几十倍。

**不含 `id`**：读者靠 slug 认内容，而 id 是跨草稿连号的，公开它等于报告有多少未发布的东西。

`category` 未分类时是 `null`，不是空对象。未分类是正常状态，不是数据缺失。嵌套对象而不是裸 id，因为读者对分类做的每件事都需要标签和链接：渲染「in 旅行」并指向 `/entries?category=travel`。它来自 `LEFT JOIN`，不额外花一次查询。

`meta` 是 `{}` 而不是 `null`：列是 NOT NULL，每个消费者都预期能在里面查键。

#### `GET /api/v1/entries/:slug`

公开。字段是列表形状加上 `content_md` 和 `updated_at`。

不含 `status` 与 `visibility`：这个端点只返回已发布的内容，那两个字段会是每个响应里的两个常量。

**这是唯一能取到 `unlisted` 的接口。** 见 4.1 的说明。

草稿、`archived`、`private`、已软删，全部 404，且**与一个从未被使用的 slug 返回完全相同的响应体**。

### 4.3 写入

#### `POST /api/v1/entries`

需登录。201。

```jsonc
{
  "title": "京都的春天",          // 必填，最长 255 字符（按字符数，非字节）
  "slug": "kyoto-spring",        // 必填，最长 255，格式见下
  "content_md": "在鸭川边...",     // 必填
  "type": "travel",              // 可选，默认 journal
  "status": "draft",             // 可选，默认 draft
  "visibility": "public",        // 可选，默认 public
  "summary": "",                 // 可选
  "cover_url": "",               // 可选
  "category_id": 3,              // 可选，0 或缺省为未分类
  "meta": {},                    // 可选，原样透传的 JSON
  "happened_at": "2023-04-03T12:00:00Z"  // 可选
}
```

**slug 格式**：小写字母与数字，用单个连字符连接。无前导、尾随或连续连字符。正则 `^[a-z0-9]+(-[a-z0-9]+)*$`。

slug 由客户端提供，**不从标题派生**：从中文标题派生需要百分号编码或者在后端引入一个拼音依赖。

**`category_id` 是 id 而不是 slug**：编辑器从一个已经知道 id 的列表里选，接受 slug 会引入一次可能与外键结果不一致的查找。

**`category_id` 不做预先校验**。先读一次仍然会和一次删除竞态，外键才是真正的保证。指向不存在的分类返回 400 且 `fields.category_id`。

`title` 按**字符数**计（`varchar(255)` 数的是字符），所以一个 200 字的中文标题不会被误拒。

响应是作者视角形状，比前台详情多 `id`、`status`、`visibility`、`category_id`、`created_at`：

```jsonc
{
  "id": 42,
  "type": "travel", "title": "京都的春天", "slug": "kyoto-spring",
  "summary": "", "content_md": "在鸭川边...", "cover_url": "",
  "status": "draft", "visibility": "public",
  "meta": {}, "word_count": 1200,
  "category_id": 3,
  "category": null,
  "happened_at": "2023-04-03T12:00:00Z",
  "published_at": null,
  "created_at": "2026-08-25T12:10:48+08:00",
  "updated_at": "2026-08-25T12:10:48+08:00"
}
```

**注意 `category_id` 有值而 `category` 是 `null`。** 这不是 bug：Postgres 的 `RETURNING` 只看见被写的那一行，所以写操作知道 id 但拿不到标签。发 `{"id":3,"name":"","slug":""}` 等于对外声明一个不可能存在的分类（`name` 是 NOT NULL 且有长度 CHECK）。**编辑器确认保存成功时读 `category_id`**，它两种情况下都有值。`category` 在后台详情读取里是有值的。

`word_count` 由服务端算，不接受客户端提交。

| 失败 | 状态 | 说明 |
|---|---|---|
| 缺必填 / JSON 不合法 | 400 | |
| slug 格式错 / 过长 | 400 | `fields.slug` |
| 标题过长 | 400 | `fields.title` |
| `type` / `status` / `visibility` 不在枚举内 | 400 | 对应 `fields` |
| slug 已被占用 | 409 | `fields.slug` |
| `category_id` 指向不存在的分类 | 400 | `fields.category_id` |

#### `PATCH /api/v1/entries/:id`

需登录。200。**字段约定见 1.7，动这个接口之前先读那一节。**

可改：`title` `slug` `summary` `content_md` `cover_url` `type` `visibility` `category_id` `meta` `happened_at`

**不做 `PUT`**。`PUT` 要求客户端回传整篇内容。两个标签页同时开一篇文章时，后保存的那个会覆盖前一个改过却没提及的字段，而且悄无声息。

**`status` 会被显式拒绝，返回 400**，不是被忽略，且错误里直接给出出路：

```jsonc
{ "fields": { "status": "use POST /entries/:id/publish or /unpublish" } }
```

忽略它会让客户端发 `{"status":"published"}` 拿到 200，然后发现文章还是草稿，且永远找不到真正负责发布的端点。发布走 4.4。

改 slug 时**不会与自己冲突**：编辑器每次连着原 slug 一起提交是常见操作，把它判成 409 会让内容没法编辑。

改 `content_md` 时 `word_count` 重算。

响应形状同 create。

#### `DELETE /api/v1/entries/:id`

需登录。**软删**，204，响应体 0 字节。

第一次 204，第二次 404：删过之后这个 id 不再对应任何可见内容。

行还在库里，只是多了一列 `deleted_at`，每个读都过滤它。所以内容是**不可达**而不是不存在。**没有硬删接口，也没有恢复接口**：内容不可再生，而恢复功能需要一个能列出已删内容的界面，那是另一个功能。

**软删会释放 slug**：唯一索引是部分索引（`WHERE deleted_at IS NULL`），所以同一个 slug 之后能被新内容占用。

### 4.4 状态迁移

三个独立端点，都是 200，都返回作者视角形状。

已实测：`publish` → `unpublish` → `publish` → `archive` 四次调用，`published_at` 逐字符相同。

#### `POST /api/v1/entries/:id/publish`

`draft` 或 `archived` → `published`。

**`published_at` 只在第一次发布时写入，之后永不移动。** 撤回再发布，这个字段仍然是最初那个值（已实测：三次发布之间隔了 2 秒，返回值逐字符相同）。

它必须与 `status` 在**同一条 UPDATE 语句**里写：数据库有 `CHECK (status <> 'published' OR published_at IS NOT NULL)`，分两步写的第一步会被打回。

#### `POST /api/v1/entries/:id/unpublish`

`published` → `draft`。**不清 `published_at`**。

#### `POST /api/v1/entries/:id/archive`

→ `archived`。**不清 `published_at`**，前台立即 404，后台按 id 仍可读。

这个端点存在的理由：不加的话 `archived` 是一个数据库接受、却没有任何路径能写进去的状态（`PATCH` 拒收 `status`，publish / unpublish 只写 `published` 和 `draft`，只有 create 能设）。

### 4.5 后台读取

两条路由挂在 `/api/v1/admin/*` 下，背后是**独立的 SQL 语句**，不是公开查询加一个布尔开关。一个切换可见性过滤的参数，离「传错一个实参就把全部草稿发出去」只有一步。

#### `GET /api/v1/admin/entries`

需登录，分页。返回**全部状态**，包括草稿，按最近编辑排序。已软删的不返回。

| 参数 | 说明 |
|---|---|
| `page` / `page_size` | 见 1.3 |
| `status` | 按状态筛，值必须在枚举内 |

`?status=stauts`（拼错）返回 **400** 且 `fields.status: "must be one of draft, published, archived"`，不是空列表。返回空列表会被读成「没有草稿」，而实际是「你拼错了」。

**目前不支持 `?category=`**，传了会被忽略。

响应是前台列表形状加 `id`、`status`、`visibility`、`created_at`、`updated_at`。仍然**不含 `content_md`**：后台列表是待办队列，正文属于详情读取。

#### `GET /api/v1/admin/entries/:id`

需登录。按 id 而非 slug，因为 slug 是被编辑的对象之一。

返回全部状态，形状同 create 的响应，但 **`category` 在这里有值**（这是读，能 join）。

---

## 5. 分类（categories）

分类是站点导航，不是内容。它**没有 owner**：认证在这里唯一决定的事情是这个请求能不能写。

也没有软删、没有可见性、没有树形嵌套。个人站的分类超过两层就是你自己都记不住的信号。

`name` 与 `slug` 都最长 **64**（比内容的 255 窄）：分类名是导航标签，一个放不进菜单的名字不是分类。`name` 按字符数计，`slug` 按字节（格式只接受 ASCII，两者结果一致）。

### `GET /api/v1/categories`

**公开**，**不分页**。

分类是导航，需要翻页的导航没人导航得了。这是一个关于数据的判断，不是遗漏：一个有 200 个分类的站点面对的是另一个问题，不是缺一个 page 参数。

因为不分页，响应信封里**没有 `meta`**。一个对永不分页的集合报告「第 1 页共 1 页」的 meta 块，是客户端必须读过去的噪音。

```jsonc
{
  "data": [{
    "id": 3,
    "name": "旅行",
    "slug": "travel",
    "description": "places and the getting there",
    "sort_order": 10,
    "entry_count": 12
  }]
}
```

`entry_count` 数的是**读者能到达的内容**：未删、已发布、public。草稿、归档、private、**以及 unlisted 都不计入**，所以这个数字与打开该分类后看到的列表一致，不会多。

空分类会以 `entry_count: 0` 出现，不会被省略（`LEFT JOIN` 而非 `INNER JOIN`）。否则一个新建的分类在有内容归入之前是不可见的。

排序是 `sort_order` 升序，同值按 id 升序。`sort_order` 是手动排序：适合读者的顺序既不是字母序也不是创建时间序。

**这里带 `id`**，与内容列表不同。内容 id 跨草稿连号，公开它会报告有多少未发布的工作；分类 id 只透露曾经创建过多少个分类，而且筛选器需要一个东西来寻址分类。

`data` 为空时是 `[]` 而不是 `null`。

### `POST /api/v1/categories`

需登录。201。

```jsonc
{
  "name": "旅行",          // 必填，最长 64 字符
  "slug": "travel",       // 必填，最长 64，格式同内容 slug
  "description": "",      // 可选
  "sort_order": 10        // 可选，默认 0
}
```

`sort_order` 的 **0 是一个真实位置**（排在最前），不是「未指定」。

响应是 Admin 形状：带 `created_at` / `updated_at`，**不带 `entry_count`**。计数属于公开列表，在这里产生它意味着每个写响应都要多做一次 join。

`created_at` 来自列默认值，`updated_at` 来自触发器。这个模块里没有任何东西盖时间戳。

| 失败 | 状态 | 说明 |
|---|---|---|
| 缺 `name` 或 `slug` | 400 | |
| slug 格式错 / 过长 | 400 | `fields.slug` |
| `name` 为空 / 过长 | 400 | `fields.name` |
| slug 已被占用 | 409 | `fields.slug` |

409 而不是 400：请求是合法的，换个 slug 就会成功，客户端能对这个差别做出反应。

### `PATCH /api/v1/categories/:id`

需登录。200。字段约定见 1.7。

可改：`name` `slug` `description` `sort_order`

`{"description": ""}` 清空描述，`{"description": null}` 什么也不做，`{"sort_order": 0}` 移到最前。

改 slug 时不与自己冲突，与另一个分类冲突返回 409。

### `DELETE /api/v1/categories/:id`

需登录。**物理删除**，204。第二次 404。

**分类里的内容不会被删，请求也不会被拒绝。它们变成未分类。**

这是外键 `ON DELETE SET NULL` 的效果，与 `entries.author_id` 的 `ON DELETE RESTRICT` 正好相反，是有意的：内容不能因为删账号而消失，但可以因为删分类而失去分类。

**这个操作没有痕迹。** 删完之后没有任何地方记得那些内容原来属于哪个分类，一次 204 可能静默改掉很多行。所以服务端为每次删除写一条日志：

```
level=INFO msg="category deleted, its entries are now uncategorised" category_id=69
```

那是这个操作留下的唯一证据。

删除后：受影响内容的 `category_id` 变 0、`category` 变 `null`、`status` 不变、前台仍然可读；用被删的 slug 再筛选得 404。

### `GET /api/v1/admin/categories`

需登录。全部分类，带时间戳，**不带计数**。编辑需要知道一个分类上次改动是什么时候；计数属于公开列表。

同样不分页，`data` 为空时是 `[]`。

### `GET /api/v1/admin/categories/:id`

需登录。单个分类，形状同 create 的响应。按 id 而非 slug，因为这是编辑表单加载的东西，而 slug 正是被编辑的对象之一。

`:id` 不是正整数（`abc`、`0`、`-1`）返回 **400 而不是 404**，且 `fields.id`。路径段根本不是一个 id，所以不存在「缺失的资源」；返回 404 读起来像「那个分类没了」，而请求从来没有指明过一个分类。

---

## 6. 探针

在根路径，不带版本前缀：探针 URL 不应该因为 API 升版而改变。

### `GET /health`

存活探针。**不碰数据库**，永远 200。它回答的是「进程还在吗」。

### `GET /health/ready`

就绪探针。ping 数据库。

```jsonc
{
  "data": {
    "status": "ok",
    "database": { "status": "ok", "latency": "253.292µs" },
    "pool": { "total_conns": 6, "idle_conns": 6, "acquired_conns": 0, "max_conns": 25 }
  }
}
```

数据库不可用时 **503** `UNAVAILABLE`。

---

## 7. 前端接入清单

写 `frontend/` 或 `admin/` 时最容易踩的几点：

1. **`fetch` 必须带 `credentials: "include"`**，否则会话 Cookie 不会被发送。同时后端 `CORS_ALLOWED_ORIGINS` 要列出你的源，不能用 `*`。
2. **清空字段用空值，不要用 `null`**。见 1.7。这是最容易写错并且不报错的一处。
3. **写响应里的 `category` 是 `null`**，读响应里才有值。确认保存成功请读 `category_id`。见 4.3。
4. **分类列表没有 `meta`**，不要按分页结构去解析。
5. **`?category=` 传 slug**，且必须处理 404：那是「链接打错了」，不是「这个分类是空的」。
6. **列表响应没有 `content_md` 也没有 `id`**。需要正文走详情，需要 id 走后台接口。
7. **不要用 `message` 做判断**，用 `code`。前者会改，后者不会。
8. 报错时把 `request_id` 带上，那一个值就能定位到对应日志行。

