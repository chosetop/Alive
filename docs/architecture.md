# Alive 架构设计方案（第一阶段）

> 本文档只描述设计，不包含业务代码。
> 十项决策已与用户对齐，文末列出仍待确认的开放问题。
>
> 路径已于 2026-08-25 统一为实现里真实的 `/api/v1/*`（探针与 `sitemap.xml`、`feed.xml` 在根路径，不带版本前缀）。标 **未实现** 的接口是设计里有、代码里还没有。字段、状态码、边界行为以 `api.md` 为准，那份是对着运行中的服务核对的。

## 0. 决策基线

后续所有设计均以这十条为硬约束，不再重复论证：

| # | 决策 | 影响面 |
|---|---|---|
| 1 | 统一 `entries` 表 + `type` + JSONB `meta` | 书影音旅行不建独立表 |
| 2 | PostgreSQL 单库，开发生产同构 | 可用 JSONB 路径查询与 GIN 索引 |
| 3 | 图片走对象存储，库内只存 key | 需要 `media` 表与凭证签发接口 |
| 4 | Markdown 只存原文，frontend SSR 渲染 | 库内无 `content_html` |
| 5 | 单账号，永不开放注册，无 RBAC | 鉴权只判断「是否站主」 |
| 6 | SEO 重要，frontend 用 SSR/SSG | Public API 必须返回 SEO 字段 |
| 7 | 统一实体命名为 `Entry` | 表 `entries`，包 `internal/entry` |
| 8 | 后端目录 `backend/` | module `github.com/p30huiwei/alive/backend` |
| 9 | 对象存储为阿里云 OSS | `storage` 接口保留，用于隔离 SDK |
| 10 | admin 走同域 `/admin` | 生产无 CORS，Cookie 用 `SameSite=Strict` |

## 1. 产品核心模型

### 1.1 先回答具体事实

你列出的记录类型，按「数据形状」而非「话题」分类，实际只有三类：

```
类型                    有正文?  有结构化属性?   有时空坐标?
─────────────────────────────────────────────────────────
人生日志 / 随笔 / 想法    有       无             时间
读过的书 / 看过的电影      有       书名作者评分    时间
听过的音乐               可有可无  曲名艺人        时间
旅行                    有       地点            时间 + 地点
照片                    可有可无  EXIF           时间 + 地点
值得记录的人和事          有       无             时间
```

关键观察：**它们的差异全部落在「结构化属性」一列，而正文、标题、slug、发布状态、时间戳完全同构。**

为每种类型建表，等于把六张 90% 相同的表并排放着，然后在 handler、service、repository 三层各写六遍近乎一致的 CRUD。这是典型的用表结构去表达「分类」——分类是数据，不是 schema。

### 1.2 抽象结论

```
                      ┌─────────────────────────┐
                      │        entries          │  ← 唯一内容实体
                      │  type: 区分记录类型      │
                      │  content_md: 我写的话    │
                      │  meta JSONB: 类型专属属性 │
                      └───────────┬─────────────┘
                                  │
        ┌─────────────┬───────────┼───────────┬──────────────┐
        │             │           │           │              │
   categories     entry_tags    media    entry_media    （未来扩展）
   （一对多）      （多对多）    （独立）   （多对多）
```

**统一进 entries（靠 type + meta 区分）**

日志、随笔、想法、书、电影、音乐、旅行、人和事，全部是 Entry。
新增「播客」「展览」「学过的课」只需新增一个 `type` 值，DDL 零改动。

**确实独立的实体**

只有三个，理由各不相同：

- `users` — 身份，与内容是完全不同的关注点。
- `categories` — 需要独立的名称、slug、排序、描述，且要能在没有任何 Entry 时先存在。
- `media` — 一张图可被多篇 Entry 复用，且有自己的生命周期（上传成功但从未被引用的孤儿文件需要能被清理）。这是「一图多用」决定的，不是「图片很重要」决定的。

**靠 meta 表达，不进 schema**

书的作者、电影的导演、音乐的专辑、旅行的坐标。

```jsonc
// type = 'book'
{ "author": "鲁迅", "rating": 4, "finished_at": "2026-03-01", "isbn": "..." }

// type = 'movie'
{ "director": "王家卫", "rating": 5, "watched_at": "2026-02-14", "year": 2000 }

// type = 'travel'
{ "place": "京都", "lat": 35.0116, "lng": 135.7681, "days": 5 }

// type = 'journal'
{}
```

### 1.3 这个抽象的代价，说清楚

JSONB 不是免费的。你会在两件事上付钱：

1. **无强制约束。** 数据库不会阻止你把 `rating` 写成字符串 `"四星"`。约束必须由 backend 的 service 层按 `type` 校验，这是应用层责任。
2. **聚合查询更啰嗦。** 「按评分排序我读过的书」写成 `ORDER BY (meta->>'rating')::int`，且需要为此建表达式索引，不像独立列那样自然。

什么时候该推翻这个决定？**当某个 type 需要被独立列表页反复筛选、排序、聚合统计时**（例如你真的想做一个豆瓣式的书架页，按评分、年份、作者三个维度交叉筛选）。那时把 `book` 从 meta 里提升为独立表，是一次目标明确的局部重构，而不是推翻整个模型。现在预先建六张表，是为一个还不存在的需求付全款。

## 2. 数据库设计

### 2.1 ER 关系

```
   users
     │ 1
     │ 作者
     │ N
   entries ──── N:1 ──── categories
     │ │
     │ └─── N:M ── entry_media ──── media
     │ N
     │
     M
  entry_tags ──── N:1 ──── tags
```

关系清单：

- `users 1—N entries`：一个用户写多条 Entry。当前单账号，此关系为未来保留。
- `categories 1—N entries`：一条 Entry 最多属于一个分类，分类可为空。
- `entries N—M tags`：经 `entry_tags` 连接。
- `entries N—M media`：经 `entry_media` 连接，用于封面外的图集。

**为什么分类是一对多，标签是多对多？**
分类回答「这条记录本质上是什么」，本质只能有一个，且分类要能作为导航结构存在。标签回答「这条记录关于什么」，可以有很多个，且标签是自由生长的，无需预先规划。两者不是同一个东西的两种实现，混用会让导航结构失控。

### 2.2 表设计

#### users

> 已由 migration `000002` 落地。下表与实际 schema 一致（`psql -d alive -c "\d users"` 核对，非凭记忆）。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | BIGSERIAL PK | 主键 |
| username | CITEXT NOT NULL | 登录名，大小写不敏感由列类型保证 |
| password_hash | TEXT NOT NULL | Argon2id PHC 串，绝不存明文 |
| role | VARCHAR(16) NOT NULL DEFAULT 'owner' | 存储但不参与任何鉴权判断 |
| display_name | VARCHAR(64) | 前台展示名，可空 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 由 `set_updated_at()` 触发器维护 |

- 唯一约束：`UNIQUE (username)`；`CHECK (length(username) BETWEEN 3 AND 64)`
- **`username` 用 `CITEXT` 而非 `VARCHAR`。** 大小写无关的唯一性由列类型保证，不靠应用层每次记得 `lower()`——漏一次就会出现两个「同名」账号。
- **`password_hash` 用 `TEXT` 不设长度。** Argon2id 编码串约 100 字符，但调参后会变长，写死长度是给未来埋雷。
- **`role` 建列但零代码读取。** 与决策 5「不建 RBAC」并存的唯一做法：中间件只判断 session 是否有效，不看 role。它的作用是给未来留一个已存在的列，避免以后加列要回填。一旦开始写 `if role == "admin"` 就是在建 RBAC，需重新对齐决策 5。
- 无 `email` 字段。单账号系统不需要找回密码流程，加了就要配邮件服务。
- 无 `avatar_url`。原设计有此字段，实现时未建：头像属于展示层数据，等真的要用时再加一列，比现在建一个空列好。
- 全部时间字段用 `TIMESTAMPTZ` 而非 `TIMESTAMP`。你会旅行，会跨时区写日志，存无时区时间戳是给未来埋雷。

#### categories

| 字段 | 类型 | 说明 |
|---|---|---|
| id | BIGSERIAL PK | 主键 |
| name | VARCHAR(64) NOT NULL | 显示名 |
| slug | VARCHAR(64) NOT NULL | URL 片段 |
| description | TEXT | 描述，可用于分类页 SEO |
| sort_order | INT NOT NULL DEFAULT 0 | 手动排序 |
| created_at / updated_at | TIMESTAMPTZ NOT NULL | 时间戳 |

- 唯一约束：`UNIQUE (slug)`
- 不做树形嵌套。个人站的分类超过两层就是你自己都记不住的信号。
- 删除是物理删除，不是软删除。分类里的内容不跟着删、也不阻止删除：`entries.category_id` 是 `ON DELETE SET NULL`，删完这些内容变成未分类。
- 这一点和 `entries.author_id` 的 `ON DELETE RESTRICT` 正好相反，是有意的：内容不能因为删账号而消失，但可以因为删分类而失去分类。
- 代价是这个操作没有痕迹：删完之后没有任何地方记得那些内容原来属于哪个分类。所以 service 层为每次删除写一条日志。

#### entries（核心表）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | BIGSERIAL PK | 主键 |
| author_id | BIGINT NOT NULL FK→users.id | 作者，`ON DELETE RESTRICT` |
| category_id | BIGINT NULL FK→categories.id | 分类，`ON DELETE SET NULL` |
| type | VARCHAR(32) NOT NULL DEFAULT 'journal' | 记录类型 |
| title | VARCHAR(255) NOT NULL | 标题 |
| slug | VARCHAR(255) NOT NULL | URL 片段，前台按此寻址 |
| summary | TEXT | 摘要，用于列表页与 `og:description` |
| content_md | TEXT NOT NULL DEFAULT '' | Markdown 原文，唯一真相 |
| cover_url | TEXT | 封面，用于列表页与 `og:image` |
| status | VARCHAR(16) NOT NULL DEFAULT 'draft' | `draft` / `published` / `archived` |
| visibility | VARCHAR(16) NOT NULL DEFAULT 'public' | `public` / `private` / `unlisted` |
| meta | JSONB NOT NULL DEFAULT '{}' | 类型专属结构化属性 |
| word_count | INT NOT NULL DEFAULT 0 | 字数，写入时算，列表页免读正文 |
| happened_at | TIMESTAMPTZ NULL | 事情**发生**的时间 |
| published_at | TIMESTAMPTZ NULL | 首次发布时间 |
| created_at | TIMESTAMPTZ NOT NULL | 记录创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 最后修改时间 |
| deleted_at | TIMESTAMPTZ NULL | 软删除标记 |

几个字段需要解释：

**`status` 与 `visibility` 为什么是两个字段？**
它们回答两个不同问题。`status` 是「这篇写完了吗」，`visibility` 是「谁能看」。合成一个枚举，你就会得到 `draft / published / published_private / published_unlisted` 这种正在失控的组合爆炸。拆开后各自都是正交的小集合。

`unlisted` 的用途：有链接的人能看，但不出现在任何列表和 sitemap 里。给朋友单独分享某篇时很有用。

**`happened_at` 为什么必须独立于 `created_at`？**
你的需求是「记录我活过」，不是「记录我什么时候打字」。补写三年前的一趟旅行时，`created_at` 是今天，`happened_at` 是三年前。前台的人生时间线必须按 `happened_at` 排，否则那条时间线记录的是你的写作习惯，不是你的人生。这个字段是整个模型里最贴合你项目定位的一个。

`happened_at` 为空时回落到 `published_at`——排序与显示用的是同一个 `COALESCE` 表达式（2026-08-25 定，见 `api.md` 的 `GET /api/v1/entries`）。一条没有发生日期的内容，读者看到的日期就是它的发布日期，那它也必须排在那个位置上。

**`published_at` 语义定死：首次发布时间。**
重新编辑已发布的内容不更新它，否则前台按发布时间排序会因为你修个错别字就把三年前的日志顶到首页。

**索引**

```sql
-- 前台列表页主查询：已发布 + 公开，按发布时间倒序
CREATE INDEX idx_entries_public_feed
  ON entries (published_at DESC)
  WHERE deleted_at IS NULL
    AND status = 'published'
    AND visibility = 'public';

-- 人生时间线：按事情发生的时间
CREATE INDEX idx_entries_timeline
  ON entries (happened_at DESC)
  WHERE deleted_at IS NULL AND status = 'published';

-- 前台列表实际用的那一个（migration 000005）。
-- 表达式索引，因为 ORDER BY 用的是 COALESCE：Postgres 只在索引表达式与排序
-- 表达式文本一致时才拿它排序，上面那个 (happened_at DESC) 顶不了这个用。
CREATE INDEX idx_entries_public_timeline
  ON entries (COALESCE(happened_at, published_at) DESC, id DESC)
  WHERE deleted_at IS NULL
    AND status = 'published'
    AND visibility = 'public';

-- 按类型筛选（书架页、影单页）
CREATE INDEX idx_entries_type
  ON entries (type, published_at DESC)
  WHERE deleted_at IS NULL AND status = 'published';

-- 分类页
CREATE INDEX idx_entries_category
  ON entries (category_id, published_at DESC)
  WHERE deleted_at IS NULL AND status = 'published';

-- 后台列表：全状态，按更新时间
CREATE INDEX idx_entries_admin
  ON entries (updated_at DESC)
  WHERE deleted_at IS NULL;

-- meta 内部查询
CREATE INDEX idx_entries_meta ON entries USING GIN (meta);
```

这批索引全部是**部分索引**（partial index），`WHERE` 条件直接写进索引定义。因为前台的每一次查询都必然带 `deleted_at IS NULL AND status='published'`，把这个条件固化进索引，索引体积只包含真正会被查的行。这是 Postgres 相对 SQLite 的一个实际优势。

**唯一约束**

```sql
-- slug 唯一，但软删除的记录不占用 slug
CREATE UNIQUE INDEX uk_entries_slug
  ON entries (slug)
  WHERE deleted_at IS NULL;
```

用部分唯一索引而非表级 `UNIQUE`。否则你删掉一篇 `/hello-world` 后，永远无法再创建同名 slug，只因为一条你已经看不见的记录还占着位置。

**约束检查**

```sql
CHECK (status IN ('draft','published','archived'))
CHECK (visibility IN ('public','private','unlisted'))
-- 已发布必须有发布时间
CHECK (status <> 'published' OR published_at IS NOT NULL)
```

`type` 加 CHECK，约束六个类型。

这里推翻了本文档早先的「type 不加 CHECK，合法值由 backend 常量维护」。原因是「零迁移」省下的成本，换来的是一类查不出的故障：没有约束时 `'joural'` 这样的拼写会正常写入，之后从所有按 type 过滤的查询里消失，而任何一层都不报错。加了约束，新增第七种类型要写一次 migration，这个成本是有意的：它把「多一种内容类型」变成一次明确的决策，而不是一次拼写。

实际实现见 `backend/migrations/000003_create_entries.up.sql` 的 `entries_type_check`。

#### tags

| 字段 | 类型 | 说明 |
|---|---|---|
| id | BIGSERIAL PK | 主键 |
| name | VARCHAR(64) NOT NULL | 显示名 |
| slug | VARCHAR(64) NOT NULL | URL 片段 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |

- 唯一约束：`UNIQUE (slug)`
- 不存 `entry_count`。这是可以随时 `COUNT` 出来的派生数据，存下来就要维护一致性，而个人站的标签数量根本不需要这层缓存。

#### entry_tags

| 字段 | 类型 | 说明 |
|---|---|---|
| entry_id | BIGINT NOT NULL FK→entries.id | `ON DELETE CASCADE` |
| tag_id | BIGINT NOT NULL FK→tags.id | `ON DELETE CASCADE` |

- 主键：`PRIMARY KEY (entry_id, tag_id)` — 复合主键天然防重复，不需要额外的 id 列。
- 补充索引：`INDEX (tag_id, entry_id)` — 支撑「某标签下的所有内容」反向查询。
- 关联表用**物理删除**级联。它不承载任何独立信息，保留一条指向已删内容的关联毫无价值。

#### media

| 字段 | 类型 | 说明 |
|---|---|---|
| id | BIGSERIAL PK | 主键 |
| object_key | VARCHAR(512) NOT NULL | 对象存储内的完整路径 |
| url | TEXT NOT NULL | 可公开访问的 URL |
| mime_type | VARCHAR(64) NOT NULL | MIME 类型 |
| size_bytes | BIGINT NOT NULL | 文件大小 |
| width / height | INT NULL | 图片尺寸，供前端预留占位防抖 |
| checksum | VARCHAR(64) | 内容哈希，用于秒传与去重 |
| uploaded_at | TIMESTAMPTZ NOT NULL | 上传时间 |

- 唯一约束：`UNIQUE (object_key)`
- `width` / `height` 不是可选的优化。列表页没有尺寸信息就无法预留空间，图片加载时页面会跳动，这个体验问题会一直跟着你。

#### entry_media

| 字段 | 类型 | 说明 |
|---|---|---|
| entry_id | BIGINT NOT NULL FK→entries.id | `ON DELETE CASCADE` |
| media_id | BIGINT NOT NULL FK→media.id | `ON DELETE RESTRICT` |
| sort_order | INT NOT NULL DEFAULT 0 | 图集内顺序 |

- 主键：`PRIMARY KEY (entry_id, media_id)`
- `media` 侧用 `RESTRICT` 而非 `CASCADE`：删一张图不该把引用它的内容一起删掉。想删图先解除引用，这个「不方便」正是它该有的保护作用。

### 2.3 删除策略

三种删除方式，按数据是否可再生来选，不是按重要性来选：

| 对象 | 策略 | 理由 |
|---|---|---|
| entries | 软删除（`deleted_at`） | 内容不可再生，误删无法恢复 |
| entry_tags / entry_media | 物理删除 + CASCADE | 纯关联，无独立信息 |
| tags / categories | 物理删除 | 可重建，Entry 侧用 `SET NULL` 兜住 |
| media | 软删除 + 孤儿清理 | 数据库记录与对象存储文件需分步处理 |
| users | 禁止删除（RESTRICT） | 单账号系统，删了整站内容失去作者 |

**软删除的纪律，这条比策略本身重要：**
所有 `entries` 查询必须带 `deleted_at IS NULL`。这个约束不能靠人记，必须在 repository 层用一个统一的基础查询构造器强制注入，让「查到已删数据」在代码结构上不可能发生。任何一处漏写，就会有一篇你以为删掉的日志出现在前台。

**media 的两阶段删除：**
数据库标记删除和对象存储删文件是两个系统，无法在一个事务里保证一致。设计上先标记数据库，对象存储的实际文件由一个独立的清理任务处理。第一阶段这个清理任务可以是一条手动执行的命令，不需要定时调度。

## 3. 后端架构

### 3.1 目录结构

```
backend/
├── cmd/
│   ├── server/main.go          # HTTP 服务入口
│   └── cli/main.go             # 建账号、清理孤儿文件
├── internal/
│   ├── config/                 # 配置加载与校验
│   ├── entry/                  # 内容领域（垂直切分）
│   │   ├── handler.go          # HTTP 出入口
│   │   ├── service.go          # 业务规则
│   │   ├── repository.go       # SQL
│   │   ├── model.go            # 领域模型
│   │   └── dto.go              # 请求响应结构
│   ├── taxonomy/               # 分类与标签
│   ├── media/                  # 媒资与上传凭证
│   ├── auth/                   # 登录与 token
│   ├── storage/                # 对象存储抽象（接口 + 实现）
│   ├── middleware/             # 鉴权、日志、恢复、CORS
│   ├── apperr/                 # 错误类型与 HTTP 映射
│   ├── httpx/                  # 响应信封、request id
│   ├── postgres/               # 连接池
│   ├── health/                 # 健康检查
│   └── router/                 # 路由注册，唯一汇聚点
├── migrations/                 # SQL 迁移文件
├── sql/queries/                # sqlc 输入
├── sqlc.yaml
└── .env.example
```

第二阶段已落地的部分：`config/`、`apperr/`、`httpx/`、`postgres/`、`middleware/`、`router/`、`health/`。业务领域目录（`entry/` `taxonomy/` `media/` `auth/` `storage/`）尚未创建。

配置采用环境变量而非 YAML：`config` 包因此零外部依赖，不需要 Viper。

### 3.2 为什么按领域切，不按层切

常见模板是 `handler/ service/ repository/` 三个顶层目录，每个里面塞所有领域的文件。改一个功能要在三个相距很远的目录之间跳，而 `handler/entry.go` 和 `handler/media.go` 明明毫无关系却是邻居。

按领域切之后，`entry/` 目录里就是关于内容的全部代码。删除一个功能等于删除一个目录。判断代码放哪，看它属于哪个领域，不需要先想「这算 service 还是 repository」。

层次没有消失，它在文件名里：`handler.go → service.go → repository.go`，依赖方向单向向下，禁止反向。

### 3.3 storage 为什么是独立包且必须是接口

```go
type Storage interface {
    PresignPut(ctx, key string, contentType string) (url string, err error)
    Delete(ctx, key string) error
    PublicURL(key string) string
}
```

供应商已定为阿里云 OSS，接口依然保留，但理由变了。原本是「推迟一个未知的选型」，现在是两条仍然成立的理由：

1. **隔离 SDK。** OSS SDK 的类型不该出现在 `media` 领域的函数签名里。它进来了，`media` 就再也无法在不连 OSS 的情况下被测试。
2. **可测试性。** 单元测试需要一个假实现来断言「签发凭证时用了正确的 key」这类逻辑，而不是真的往 OSS 上传文件。

第二条是接口在这里的主要价值。换供应商只是附带好处。

### 3.4 错误处理

一个统一的应用错误类型，只在最外层中间件转成 HTTP 响应：

```go
type AppError struct {
    Code    string  // ENTRY_NOT_FOUND
    Message string  // 给人看的
    Status  int     // HTTP 状态码
    cause   error   // 内部原因，只进日志，绝不出响应
}
```

原则：**service 层不知道 HTTP 的存在。** 它返回 `apperr.NotFound(...)`，由中间件决定这对应 404。这样同一份业务逻辑将来给 CLI 或 gRPC 用时不需要改动。

`cause` 永远不进响应体。数据库报错原文进了响应，就是把表名字段名送给攻击者。

## 4. API 设计

### 4.1 通用约定

**Public API 全部无需认证，Admin API 全部需要站主身份。** 单账号系统不存在中间状态。

响应外层统一：

```jsonc
// 成功
{ "data": { ... } }

// 列表
{ "data": [ ... ], "pagination": { "page": 1, "page_size": 20, "total": 137 } }

// 失败
{ "error": { "code": "ENTRY_NOT_FOUND", "message": "内容不存在" } }
```

错误码表：

| HTTP | code | 触发场景 |
|---|---|---|
| 400 | `INVALID_PARAM` | 参数格式或取值非法 |
| 401 | `UNAUTHORIZED` | 无 token 或 token 失效 |
| 404 | `ENTRY_NOT_FOUND` | 目标不存在，或存在但无权见 |
| 409 | `SLUG_CONFLICT` | slug 已被占用 |
| 422 | `INVALID_STATE` | 状态流转非法，如重复发布 |
| 500 | `INTERNAL_ERROR` | 未预期错误，细节只进日志 |

**404 覆盖「无权访问」。** 对未认证请求，草稿和不存在的内容返回完全相同的响应。用 403 会泄露「这个 slug 确实有内容」，等于把你的草稿标题空间暴露给探测。

### 4.2 Public API

#### `GET /api/v1/entries`

内容列表。

| 参数 | 类型 | 说明 |
|---|---|---|
| page | int | 默认 1 |
| page_size | int | 默认 20，上限 50 |
| type | string | 按类型筛选 |
| category | string | 分类 slug |
| tag | string | 标签 slug |
| sort | string | `published_at` (默认) / `happened_at` |

只返回 `status=published AND visibility=public AND deleted_at IS NULL`。

`category` 传的是 slug，但过滤在 SQL 里用的是 id：请求先把 slug 解析成 id，解析不到就是 404，不是空列表。`?category=nope`（打错的链接）和 `?category=一个还没有内容的分类`（空分类）是两回事，客户端分不清就会把前者显示成「这个分类下暂无内容」。

`category` 传空串（`?category=`）等同于没传。前端从表单状态拼查询串时，分类选「全部」就会发出 `category=`，对这种请求报 400 会打断这个筛选器唯一要服务的场景。

分类筛选只会收窄可见范围，不会放宽：加了 `category` 之后，上面那三个条件一个都不少。

响应**不含 `content_md`**。列表页不需要正文，返回它会让首屏响应膨胀几十倍。

```jsonc
{
  "data": [{
    "slug": "kyoto-spring",
    "type": "travel",
    "title": "京都的春天",
    "summary": "在鸭川边坐了一整个下午。",
    "cover_url": "https://cdn.example.com/xxx.webp",
    "category": { "name": "旅行", "slug": "travel" },
    "tags": [{ "name": "日本", "slug": "japan" }],
    "meta": { "place": "京都" },
    "word_count": 1820,
    "happened_at": "2026-04-03T00:00:00Z",
    "published_at": "2026-04-10T12:30:00Z"
  }],
  "pagination": { "page": 1, "page_size": 20, "total": 137 }
}
```

#### `GET /api/v1/entries/:slug`

内容详情。返回完整字段，含 `content_md` 与 `media` 图集。

`visibility=unlisted` 的内容**可以**通过此接口取到（有链接即可访问），但不出现在列表与 sitemap。

#### 其余 Public 接口

| Method | URL | 说明 |
|---|---|---|
| GET | `/api/v1/categories` | 分类列表，含内容计数 |
| GET | `/api/v1/tags` | 标签列表，含内容计数（**未实现**） |
| GET | `/api/v1/archives` | 按年月聚合的归档，用于时间线（**未实现**） |
| GET | `/api/v1/site` | 站点信息与作者资料（**未实现**） |
| GET | `/sitemap.xml` | 见 5.6 节的归属讨论（**未实现**） |
| GET | `/feed.xml` | RSS（**未实现**） |

### 4.3 需登录的接口

**`/api/v1/admin/*` 前缀只给读，不给写。** 这一点本节早先写反了，纠正如下。

写操作在 `/api/v1/entries`，与公开读同一个路径，靠方法和鉴权中间件区分：`GET /entries` 公开，`POST /entries` 需登录。而**能返回草稿的读**挂在 `/api/v1/admin/entries` 下，背后是独立的 SQL 语句。

分界线是「这条语句能不能看见未发布的内容」，不是「这个操作要不要登录」。前者是数据可见性，写错了会把草稿发出去；后者中间件已经管了。一个切换可见性过滤的布尔参数，离「传错一个实参就把全部草稿发出去」只有一步，所以那批语句连参数都不共享。

#### `POST /api/v1/auth/login`

```jsonc
// 请求
{ "username": "...", "password": "..." }

// 响应
{ "data": { "id": 1, "username": "owner", "role": "owner", "display_name": "" } }
```

**响应体里没有 token。** 会话凭证只存在于 `Set-Cookie` 头里，库里存的是它的 SHA-256。没有 access token，也没有 `expires_in`：这不是双 token 方案。理由见 5.4 节。

| Method | URL | 说明 |
|---|---|---|
| POST | `/api/v1/auth/logout` | 删除会话行并清 Cookie，幂等 |
| GET | `/api/v1/me` | 当前会话对应的账号 |
| POST | `/api/v1/entries` | 创建，默认落 `draft` |
| PATCH | `/api/v1/entries/:id` | 局部更新，供编辑器自动保存 |
| DELETE | `/api/v1/entries/:id` | 软删除 |
| POST | `/api/v1/entries/:id/publish` | 发布 |
| POST | `/api/v1/entries/:id/unpublish` | 撤回为草稿 |
| POST | `/api/v1/entries/:id/archive` | 归档 |
| GET | `/api/v1/admin/entries` | 列表，可按 status 筛，含草稿 |
| GET | `/api/v1/admin/entries/:id` | 详情，按 id 而非 slug |
| POST | `/api/v1/categories` | 创建分类 |
| PATCH | `/api/v1/categories/:id` | 局部更新 |
| DELETE | `/api/v1/categories/:id` | 物理删除，内容变未分类 |
| GET | `/api/v1/admin/categories` | 列表，带时间戳，不带计数 |
| GET | `/api/v1/admin/categories/:id` | 详情，编辑表单加载用 |
| POST | `/api/v1/admin/media/presign` | 换取上传直传凭证（**未实现**） |
| POST | `/api/v1/admin/media` | 上传完成后登记元数据（**未实现**） |

**没有 `PUT`。** 本节早先列了「`PUT /:id` 全量更新」，2D 阶段明确否掉：`PUT` 要求客户端回传整篇内容，两个标签页同时开一篇文章时，后保存的那个会覆盖前一个改过却没提及的字段，而且悄无声息。

**没有 `/auth/refresh`。** 那是被删掉的 JWT 双 token 方案的残留。会话是 7 天滑动过期，剩余寿命不足一半时由中间件顺延，客户端不需要做任何事。

`archive` 是 2D 阶段加的第三个状态迁移端点。不加的话 `archived` 是一个数据库接受、却没有任何路径能写进去的状态。

**后台按 `id` 寻址，前台按 `slug` 寻址。** 编辑标题时 slug 可能改变，用 slug 寻址会让「改标题」这个动作变成「换了一个资源」。

#### 为什么发布是独立接口而非 `PUT status='published'`

发布不是改一个字段，它是一次状态流转，附带若干规则：

1. 校验 `title` 与 `content_md` 非空
2. `published_at` 为空时填入当前时间，非空时保持不变
3. 校验 slug 在已发布内容中唯一

把这些规则塞进通用 `PUT`，意味着每次保存草稿都要跑一遍发布校验，或者在 `PUT` 里写一堆 `if status changed` 分支。独立接口让「保存」和「发布」在代码里就是两件事——这正是它们在你脑子里的样子。

`unpublish` 同理：它把 `status` 退回 `draft`，但**保留 `published_at`**。重新发布时首发时间不丢。

## 5. 三端职责边界

### 5.1 数据流

```
        ┌──────────────┐
        │   访客浏览器   │
        └──────┬───────┘
               │ ① HTML 请求
               ▼
        ┌──────────────┐   ② Public API (无认证)
        │   frontend   │──────────────────┐
        │  Nuxt SSR    │◄─────────────────┤
        │              │   ③ JSON         │
        └──────┬───────┘                  │
               │ ④ 完整 HTML + meta        │
               ▼                          ▼
        ┌──────────────┐          ┌──────────────┐
        │   访客浏览器   │          │   backend    │
        └──────────────┘          │  Go + Gin    │
                                  │              │
        ┌──────────────┐          │ 唯一数据权威   │
        │   你的浏览器   │          │ 唯一鉴权边界   │
        └──────┬───────┘          └──┬────────┬──┘
               │ SPA                 │        │
               ▼                     │        │
        ┌──────────────┐  Admin API  │        │
        │    admin     │─────────────┘        │
        │  Vite + Vue  │  Cookie 会话          │
        └──────┬───────┘                      ▼
               │                       ┌──────────────┐
               │  图片直传（预签名）      │  PostgreSQL  │
               └──────────────────────►└──────────────┘
                        ┌──────────────┐
                        │  对象存储 CDN  │
                        └──────────────┘
```

### 5.2 各端能做什么，不能做什么

**backend**

- 负责：数据持久化、鉴权、状态流转规则、slug 唯一性、上传凭证签发。
- 不负责：渲染 Markdown、生成 HTML 页面、生成 meta 标签、图片裁剪压缩。
- 铁律：**它是唯一决定「这条内容能不能被这个请求者看到」的地方。** 过滤逻辑绝不允许出现在 frontend。前端做的任何过滤都只是显示效果，不是安全边界。

**frontend**

- 负责：SSR 渲染、Markdown 转 HTML、meta 标签、sitemap、RSS、路由、缓存。
- 不负责：任何写操作，不持有任何凭证。
- 铁律：**它只调 Public API，不持有 token。** 一旦它需要 token 才能拿到某些数据，说明 Public API 的边界设计错了。

**admin**

- 负责：编辑体验、Markdown 预览、草稿自动保存、图片直传对象存储。
- 不负责：判断权限（只根据 API 返回的 401 决定跳登录页）。
- 铁律：**它是纯 SPA，不需要 SSR。** 只有你一个人用，SEO 无意义，加 SSR 纯属自找麻烦。

「不判断权限」这一条在 3A 落地为一条具体规则：登录态的唯一依据是 `GET /api/v1/me` 的响应，不存任何本地标记。一个持久化的 `isLoggedIn=true` 会活得比它描述的会话更久，于是界面渲染出一副已登录的外壳，而其中每个请求都 401。详见 `progress.md` 第 10 节。

#### 5.2.1 前台 URL 结构（2026-08-25 定）

| 页面 | 路径 | 文件 |
|---|---|---|
| 列表 | `/`、`/?page=N` | `pages/index.vue` |
| 详情 | `/<slug>` | `pages/[slug].vue` |
| 分类 | `/categories/<slug>` | `pages/categories/[slug].vue` |

详情页在**根路径**下，不是 `/entries/<slug>`。理由是这是个人站，`entries` 这一段不承载任何信息，只是把每个链接都变长。

代价必须写下来：**slug 与静态路径同名时，那篇内容将无法访问。** Nuxt 里静态路由优先于动态路由，所以叫 `categories` 的文章打不开。目前被占用的段只有 `categories`。新增任何顶层静态路由，都是在从内容的命名空间里划走一个词。

分页用 `?page=`，不是 `/page/N`。第 2 页起标 `noindex, follow`：列表页和它链向的文章争同一批关键词，而应该被搜到的是文章。

#### 5.2.2 主题切换（2026-08-25 定）

两套配色：`ink`（墨与纸，浅）与 `lamp`（灯下，深）。

- 值只写一遍，用 CSS `light-dark()`。写两遍是两套主题日后走偏的标准做法。
- 选择存在 **cookie**，不是 `localStorage`。服务端渲染第一个字节时能读到 cookie，读不到 `localStorage`；存后者意味着每次加载都先渲染错的配色、水合之后再改回来，即肉眼可见的闪烁。
- cookie 未设置时**不写** `data-theme`，于是 `light-dark()` 跟随操作系统。首访就写一个默认值会覆盖掉读者已经在系统里表达过的偏好。
- 未知或恶意的 cookie 值一律当作未设置，不写进 HTML。

后台配置主题是后续能力，`tokens.css` 里新增一个 `[data-theme]` 块即可，无需改任何组件——前提是组件只通过变量读颜色。**组件里出现硬编码颜色，等于一套切不动的主题。**

### 5.3 一个必须遵守的约束

frontend 与 admin 共用一份 Markdown 渲染配置，抽成独立的共享包或 git submodule。

否则会发生：你在 admin 里预览得好好的表格，发布后在 frontend 上变成一坨纯文本，因为两边插件配置不一致。这是「frontend 渲染」决策必须付的配套成本，不做这件事，那个决策就是有缺陷的。

#### 5.3.1 这条约束目前无法字面满足（2026-08-25）

选定 Milkdown 之后，这条约束不再可能按字面执行，必须记下来，而不是当它还成立：

- frontend 用 `markdown-it` 把 Markdown 源文本渲染成 HTML；
- admin 的 Milkdown 走 ProseMirror，用自己的 schema 解析，不经过 `markdown-it`。

两边没有一份可以共享的配置对象。共享包能装的只有「同一个 `markdown-it` 实例」，而 Milkdown 用不上它。

实际风险比原文小，但不是零：两边都覆盖 CommonMark + GFM，重叠面很宽。真正会出事的是**单边扩展**——只在 Milkdown 装的语法，发布后在前台就是纯文本；只在 `markdown-it` 装的插件，在编辑器里看不到效果。

因此约束改成一条可执行的规则：

> **两侧都只使用 CommonMark + GFM 范围内的语法。任何一侧新增语法扩展，必须同时在另一侧实现，否则不许合并。**

代码里的说明在 `frontend/utils/markdown.ts` 顶部。

### 5.4 认证方案

> **已定：服务端 session，不用 JWT。已于 2026-08-24 实现。**
>
> 原方案 B（JWT 双 token）已删除。删除理由记在本节末尾，因为「为什么没选 JWT」比「选了什么」更容易被后来的人重新提出。
> 实施细节见 `stage-auth-plan.md`，当前状态见 `progress.md`。

```
登录成功 ──► crypto/rand 生成 32 字节 token
          ──► 库内插入 sessions 行，只存 SHA-256(token)
          ──► Set-Cookie: alive_session=<token>
                HttpOnly, Secure, SameSite=Strict, Path=/api/v1
每次请求 ──► 中间件读 cookie → SHA-256 → 查库（join users）
          ──► 剩余寿命不足一半时顺延 expires_at
登出     ──► DELETE 该行，立即失效
```

关键取值与理由：

| 项 | 取值 | 理由 |
|---|---|---|
| token | 32 字节 `crypto/rand` | 熵足够，不可预测 |
| 库内存储 | SHA-256 摘要，非明文 | 库被读走也无法重放成 cookie |
| 密码 | Argon2id，PHC 串编码 | bcrypt 静默截断 72 字节且无内存成本 |
| cookie | `HttpOnly` `SameSite=Strict` `Path=/api/v1` | Strict 是同域 `/admin` 换来的；子域方案只能用 Lax |
| 过期 | 7 天滑动，过半才续期 | 每请求续期会把每次读变成一次写 |
| 限流 | 5 次/分钟/IP，仅登录端点 | 见下 |

同域方案让 `SameSite=Strict` 可用，这是最严格取值，CSRF 面基本关闭，因此**本阶段不引入 CSRF token**——加了是重复防护。

单账号系统不需要「同时管理多设备会话」，一张 session 表就够。

**为什么不用 JWT。** JWT 的核心价值是无状态校验，让多个服务实例不必共享 session 存储。这里只有一个 Go 进程和一个数据库，这个价值拿不到，但双 token 的复杂度要全额支付。更关键的是吊销：JWT 本身无法吊销，要实现「登出后立即失效」就必须再建一张黑名单表——那时它已经有状态了，只是把 session 表换了个名字，还多背一层签名验证。服务端 session 的吊销是 `DELETE` 一行，天然的，不是补上去的。

**登录接口必须限流。** 单账号系统只有一个用户名，等于攻击者已经知道一半凭证，剩下的是纯暴力破解。实现为进程内固定窗口计数器，不引入 Redis：限流器依赖一个会宕的网络服务，就意味着它宕机时连带拖掉登录。

限流还有一层不那么直观的作用：每次登录尝试要跑一次约 200ms 的 Argon2id。没有限流，攻击者发一个 HTTP 包就能买走服务端 200ms CPU，这本身就是一种放大攻击。**因此拒绝必须发生在 handler 之前**，实测被拒请求耗时 54µs 对比正常失败 110ms。

### 5.5 图片管理

已定：阿里云 OSS。

```
admin ──① POST /media/presign ──► backend（签发凭证，不碰二进制）
      ◄── 预签名 URL ───────────┘
      ──② PUT 二进制 ──────────► 阿里云 OSS
      ──③ POST /media 登记 ────► backend ──► PostgreSQL（只存 object key）
```

图片二进制**不经过 backend**。让 Go 服务代理图片上传，等于用你的应用服务器带宽和内存去做对象存储本来就做得更好的事。

`storage` 接口在供应商确定后依然保留，但它的价值变了：不再是「推迟选型」，而是**隔离 SDK 与可测试性**。OSS SDK 不该出现在 `media` 领域的代码里，且单元测试需要一个假实现，否则测 media 逻辑要连真实的 OSS。

**OSS 特有的一件好事：** 它自带图片处理服务（IMG），缩略图、裁剪、格式转换可以通过 URL 参数完成，例如 `?x-oss-process=image/resize,w_800/format,webp`。这意味着：

- 不需要在浏览器端转 WebP，也不需要 backend 引入图像处理依赖。
- 库里只存一份原图的 key，各种尺寸由 URL 参数派生，不需要为每个尺寸存一条记录。

代价是这些 URL 参数是 OSS 专有语法，写进 frontend 就形成了供应商绑定。建议把它收在 frontend 的一个 `imageURL(key, opts)` 函数里，换供应商时只改这一个函数。

### 5.6 SEO 归属

全部在 frontend，backend 只提供原料。

| 事项 | 归属 | 说明 |
|---|---|---|
| `<title>` / `description` | frontend | 从 API 的 title / summary 生成 |
| Open Graph / Twitter Card | frontend | 用 cover_url 作 `og:image` |
| 结构化数据 JSON-LD | frontend | 按 type 映射 Article / Review |
| 语义化 HTML | frontend | Markdown 渲染时保证标题层级正确 |
| `sitemap.xml` | frontend 生成 | 需遍历全部内容，见下方开放问题 |
| `robots.txt` | frontend | 静态文件 |
| `canonical` | frontend | 防止分页与筛选产生重复内容 |

backend 的责任仅有一条：**Public API 必须提供 SEO 所需的全部字段**，即 `summary`、`cover_url`、`published_at`、`updated_at`。少任何一个，frontend 就只能拿正文截断来凑，效果会明显打折。

## 6. MVP

### 6.1 范围

严格对应你列的八条，不多做：

| # | 能力 | 涉及 |
|---|---|---|
| 1 | 登录后台 | `POST /api/v1/auth/login` + 鉴权中间件 |
| 2 | 创建 Markdown 日志 | `POST /api/v1/entries` |
| 3 | 保存草稿 | `PATCH /api/v1/entries/:id` |
| 4 | 发布日志 | `POST /api/v1/entries/:id/publish` |
| 5 | 编辑日志 | `GET /api/v1/admin/entries/:id` + `PATCH /api/v1/entries/:id` |
| 6 | 删除日志 | `DELETE /api/v1/entries/:id` |
| 7 | 前台列表 | `GET /api/v1/entries` |
| 8 | 前台详情 | `GET /api/v1/entries/:slug` |

第 5 条原先写作 `GET` + `PUT /api/admin/entries/:id`，两处都不对：写操作不在 `/admin` 下（见 4.3），并且没有 `PUT`。编辑是「后台按 id 读 + 局部更新」两个不同前缀的调用。

### 6.2 表：全建，还是只建需要的？

**全部七张表在第一个 migration 里建完，但 MVP 只操作 `users` 和 `entries`。**

理由：改表结构的代价远高于建一张暂时空着的表。而且完整的 schema 能让你在写第一行代码时就看清整个模型，避免为了「先跑起来」做出与最终结构冲突的设计。

MVP 阶段 `entries` 的行为简化为：

- `type` 恒为 `'journal'`
- `meta` 恒为 `{}`
- `category_id` 恒为 `NULL`
- `visibility` 恒为 `'public'`
- 不启用 tags 与 media

字段都在，只是暂时不填。

### 6.3 明确不做

登录之外的用户功能、注册、找回密码、评论、搜索、图片上传界面、书影音类型、多级分类、访问统计、定时发布、版本历史、多语言、Docker 编排、CI/CD。

其中要单独说的：**图片上传 MVP 不做界面。** `media` 表和 presign 接口的设计已经定稿，但第一版你可以先手动往对象存储放图、把 URL 粘进 Markdown。这让 MVP 少一整块前端工作，而设计已经为它留好位置。

### 6.4 建议的推进顺序

```
1. migrations + 数据库连接 + config
2. cmd/cli 建账号            ← 没有账号，登录接口无法测
3. auth：login + 中间件
4. entry：repository → service → handler
5. Public API 两个只读接口
6. admin：登录页 + 列表页 + 编辑器
7. frontend：列表页 + 详情页 + SSR meta
```

第 2 步容易被跳过，但没有它，第 3 步就没法验证。

## 7. 技术选型

| 领域 | 选择 | 理由 |
|---|---|---|
| Web 框架 | Gin | 已定。生态成熟，中间件模型简单 |
| 数据库 | PostgreSQL 16+ | 已定。JSONB + GIN + 部分索引 |
| 数据库访问 | **sqlc** | 见下方说明 |
| 迁移 | golang-migrate | 纯 SQL 文件，与 sqlc 天然协同 |
| 认证 | 服务端 session + HttpOnly Cookie | 见 5.4。原定 JWT 双 token 已否决 |
| 密码哈希 | Argon2id（`golang.org/x/crypto/argon2`） | 见 5.4。原定 bcrypt 已否决 |
| 配置 | env + 结构体，零依赖 | 已实现。环境变量够用，不引入 Viper |
| Markdown | frontend `markdown-it` | 已定。backend 不需要 Markdown 库 |
| 图片 | **阿里云 OSS** + storage 接口 | 已定。接口用于隔离 SDK 与可测试性 |
| frontend | **Nuxt 4** | 已定，2026-08-25 再次确认（`progress.md` 曾有一处误写 Astro，已改）。SEO 重要，Vue 生态的 SSR 就是它 |
| admin | Vite + Vue 3 + TS | **已实现**（2026-08-25）。加 vue-router + Pinia，无 UI 库。纯 SPA，挂在 `/admin` 路径下 |
| 编辑器 | **Milkdown 7.22.1** | 已定，2026-08-25。所见即所得，但存的仍是 Markdown 源文本。peer dep `vue: ^3.0.0`，与 admin 的 Vue 3.5.41 兼容 |
| 部署 | 单机 + Caddy 反代，**同域三上游** | 见下方说明 |

### 7.1 为什么推荐 sqlc 而不是 GORM

这是本方案里唯一一个我明确反对主流选择的地方，理由要说清楚。

sqlc 的工作方式：你写 SQL，它读你的 schema，生成类型安全的 Go 函数。

对本项目的三个具体好处：

1. **JSONB 查询你必须手写 SQL。** GORM 处理 `meta->>'author'`、`USING GIN` 这类 Postgres 特有语法时会退化成 `db.Raw()`。既然核心查询终究要手写 SQL，ORM 的抽象就只剩下负担。
2. **部分索引要求 SQL 精确可控。** 前面设计的六个部分索引，只有 WHERE 条件与索引定义完全匹配时才会命中。ORM 拼出的 SQL 你无法保证这一点，而且它出问题时你要先反推它生成了什么。
3. **学习价值方向不同。** 你会真正学会 SQL 和 Postgres，而不是学会一个 ORM 的 API。前者的知识十年后还有效。

代价，说实话：写得比 GORM 慢，关联查询要自己组装结构体。对七张表的项目，这个代价可以承受。

如果你更看重开发速度，GORM 是完全合理的选择，只是要接受在 JSONB 相关查询上频繁落到 Raw SQL。**这一条我保留意见但不坚持，你的项目你定。**

### 7.2 部署

已定：admin 走同域路径 `/admin`，不用独立子域。三个上游由 Caddy 按路径分发，浏览器看到的始终是同一个 origin。

```
                    ┌──────────────┐
   访客 / 站主 ─────►│    Caddy     │  自动 HTTPS，唯一 origin
                    └──┬────┬───┬──┘
              /        │    │   │        /api/*
                       │    │   └──────────────┐
                       │    └── /admin ──┐     │
                       ▼                 ▼     ▼
              ┌──────────┐      ┌────────┐  ┌──────────┐
              │  Nuxt    │      │ admin  │  │ Go 二进制 │
              │  :3000   │      │ 静态文件│  │  :8080   │
              └──────────┘      └────────┘  └────┬─────┘
                                                 ▼
                                          ┌──────────┐
                                          │ Postgres │
                                          └──────────┘
                                                 │
                                     图片直传，不经过 backend
                                                 ▼
                                          ┌──────────┐
                                          │ 阿里云 OSS │
                                          └──────────┘
```

**同域方案换来三件事：**

1. **生产环境不需要 CORS。** 跨源请求根本不发生，`CORS_ALLOWED_ORIGINS` 在生产配成空。开发环境仍需要，因为三个 dev server 在不同端口上。
2. **Cookie 可以用 `SameSite=Strict`。** 这是最严格的取值。子域方案下 `admin.域名` 向 `域名/api` 发请求属于跨站，只能退到 `Lax` 或 `None`。
3. **Cookie 不需要 `Domain` 属性。** 设了 `Domain=.域名` 会把 refresh token 泄露给所有子域，包括你以后可能加的任何一个。

代价是 Caddy 要按路径分发三个上游，配置比单上游长几行。这个代价换上面三条，值得。

单台小服务器，Caddy 做反代和自动证书，Postgres 用 Docker 跑，Go 编译成单文件二进制，Nuxt 用 Node 跑，admin 打包成静态文件由 Caddy 直接伺服。

第一阶段不上 K8s、不上 CI/CD。一个 shell 脚本完成构建和重启就够了。这不是偷懒，是把复杂度花在你真正需要的地方。

## 8. 开放问题

**已确认，不再是开放问题**

| 问题 | 结论 | 影响 |
|---|---|---|
| 对象存储 | 阿里云 OSS | storage 接口保留，OSS 为第一个实现 |
| 域名结构 | 同域 `/admin` | 生产无 CORS，Cookie 用 `SameSite=Strict` |
| 统一实体命名 | `Entry` | 表 `entries`，包 `internal/entry`，URL `/api/v1/entries` |
| 后端目录名 | `backend/` | Go module `github.com/p30huiwei/alive/backend` |
| 认证方案 | 服务端 session，非 JWT | 见 5.4 节，已实现 |
| frontend 框架 | **Nuxt**，非 Astro | 2026-08-25 确认。差别不只是名字：Nuxt 带 Node 运行时，部署要一个进程；Astro 默认零 JS 静态输出。SSR 渲染 Markdown 这条依赖前者 |
| 密码哈希 | Argon2id，非 bcrypt | 见 5.4 节与 `stage-auth-plan.md` 第 7 节 |
| admin 编辑器 | **Milkdown 7.22.1** | 2026-08-25 确认。所见即所得，存的仍是 Markdown 源文本。只影响 admin 的编辑体验，不影响后端与前台 |
| `role` 列 | 建列但零代码读取 | 与决策 5「不建 RBAC」并存的唯一做法。**admin 的 TS 类型据此把 `role` 写成 `string` 而非联合类型**：该列无 CHECK 约束，联合类型会承诺 schema 没有承诺的事 |

三端目录已统一为 `frontend/`、`admin/`、`backend/`。原 `back/`、`front/` 两个空目录已处理。三端**均已有代码且实测可同时运行**（2026-08-25：后端 :8080、admin :5173、frontend :3000），见 `progress.md` 第 11 节。

技术栈表里的 frontend 一行写「Nuxt 4」，而 `frontend/package.json` 锁的是 `nuxt: 3.21.11`。**以 package.json 为准还是以本表为准，这一条未决** —— 如果当初定的确实是 Nuxt 4，那么现在装的是错版本，该改的是依赖不是文档。见 `progress.md` 第 4 节。

**4A 之后这条的代价变了（2026-08-25）：** 前台三个页面、七个 composable、四个组件全部按 Nuxt 3 的**根级目录约定**写成（`pages/`、`composables/`、`utils/` 在项目根下）。Nuxt 4 的默认布局把这些移到 `app/` 下。所以现在改版本不只是升个依赖，还要搬整棵目录树。4A 期间已经踩过一次：`useEntryType.ts` 曾按 Nuxt 4 的习惯写进 `app/composables/`，不生效。

**要改就趁早**，越往后写代价越高。反过来说，如果决定就用 Nuxt 3，把技术栈表那一行改掉即可，代码不用动。

**仍待确认**

1. **服务器在哪？** 国内需备案，海外访问国内慢。OSS 已定为阿里云，服务器同样放国内在延迟上更一致。影响部署细节，不影响架构。
3. **各 `type` 的 meta 结构。** `type` 的取值本身已定：六值一次定齐并由 CHECK 约束限定（见 `progress.md` 第 7 节）。仍未定的是 `book` / `movie` / `music` / `travel` / `photo` 各自的 meta 形状，目前只实现了 `journal` 的。
4. **session 要不要绝对上限？** 目前只有 7 天滑动过期。一个每天都在用的 session 可以无限续下去，即 token 泄露且攻击者保持活跃时它永不自动失效。加上限需要 `sessions.absolute_expires_at` 与一个新 migration。个人站风险有限，暂按不加处理。
5. **Argon2id 参数是否下调？** 实测 212ms/次（m=64MiB, t=3, p=2）。OWASP 推荐约 50ms（m=46MiB, t=1, p=1）；`t=3` 是主要成本，降到 `t=1` 约 70ms 且仍在推荐线上。登录频率极低，倾向保持不变。

原第 4 条「认证用 JWT 还是服务端 session」已确认为服务端 session，5.4 节已重写，方案 B 已删除。

---

*本文档随决策变更更新。第二阶段（后端基础设施）与 Stage 1（users + auth）已落地：登录、登出、`/api/v1/me`、会话续期、登录限流、`user:create` 与 `session:prune` 均可用。*
