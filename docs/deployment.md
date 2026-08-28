# Alive 部署与恢复手册

Plan 5 的目标是让 Alive 作为普通个人网站分享给朋友。这里没有访客登录、邀请码、站点密码或额外访问校验；文章仍按 `public`、`unlisted`、`private` 的既有语义工作。OSS 媒体上传、Tags 和批量内容管理属于 Plan 6 或后续计划。

## 生产拓扑

```
Internet
   │ HTTPS
   ▼
Reverse proxy (Caddy/Nginx)
   ├── /          → frontend/.output (Nuxt SSR, 127.0.0.1:3000)
   ├── /admin/    → admin/dist (静态文件，SPA fallback 到 dist/index.html)
   ├── /api/*     → backend (127.0.0.1:8080)
   └── /health*   → backend (liveness/readiness)
                         │ private network only
                         ▼
                    PostgreSQL
```

HTTPS 在反向代理终止；Go API 不直接暴露到公网。PostgreSQL 只允许来自 API 主机或私有网络的连接。代理转发到 API 的网络地址必须写入 `TRUSTED_PROXIES`，不能填 `0.0.0.0/0`。

代理的等价路由配置应满足以下约束（域名和目录按部署机替换）：

```text
example.com/        → 127.0.0.1:3000
example.com/admin/* → /srv/alive/admin/dist/*，不存在的文件回退到 dist/index.html
example.com/api/*   → 127.0.0.1:8080/api/*
example.com/health* → 127.0.0.1:8080/health*
```

生产构建的 admin 必须以 `/admin/` 为 base path；本地开发仍使用 `http://localhost:5173`。Nuxt 生产进程继续监听本机端口，代理只把公开页面交给它。

## 生产配置

从 `backend/.env.example` 复制配置模板，通过 systemd、容器编排或部署平台的 secret/environment 管理注入值；不要把 `.env`、数据库密码、AccessKey、Token 或 cookie 值写进仓库、镜像或日志。

必须确认：

```text
APP_ENV=production
SERVER_HOST=127.0.0.1                 # 或仅 API 私网地址
SERVER_PORT=8080
DATABASE_URL=postgres://...           # 仅由部署 secret 提供
CORS_ALLOWED_ORIGINS=                 # 同源 /admin 与 /api，不需要跨域
SESSION_COOKIE_SECURE=true
SESSION_LIFETIME=168h
SESSION_ABSOLUTE_LIFETIME=720h        # 30 天硬上限
RATE_LIMIT_ENABLED=true
TRUSTED_PROXIES=10.0.0.0/8            # 只填实际代理网段或 IP
```

可先运行 `make config-check`。它只报告解析后的非敏感配置摘要，数据库 DSN 只报告长度，不连接数据库，也不会打印密码。生产启动顺序是：数据库可连接 → `make migrate-up` → `make config-check` → 启动 API → 检查探针 → 启动/切换 frontend 与 admin → 执行发布 smoke test。

`GET /health` 是不访问数据库的存活探针；`GET /health/ready` 会 ping PostgreSQL，数据库不可用时返回 503。生产日志由 Go `slog` 输出 JSON 到 stdout，等级为 INFO；由 systemd journal、容器日志驱动或平台日志系统收集。不要在代理访问日志中记录 cookie 或 Authorization 内容。

回滚先停止接收新流量并恢复上一版 frontend/admin/backend；数据库 migration 只能在有对应 down migration 且确认兼容时回滚，不能用代码回滚替代数据库恢复。Plan 5 的恢复路径是将备份恢复到隔离数据库，确认版本与数据后，再按经过审核的窗口恢复生产库。

## 本地三端工作流

```bash
# API
cd backend
make migrate-up
make run                         # http://localhost:8080

# admin
cd ../admin
npm run dev                      # http://localhost:5173

# public frontend
cd ../frontend
npm run dev                      # http://localhost:3000
```

本地 API 使用 `SESSION_COOKIE_SECURE=false`，并保持一个 hostname（推荐全部使用 `localhost`）；`localhost` 与 `127.0.0.1` 的 host-only cookie 不互通。

## PostgreSQL 备份

使用 custom format，便于选择性恢复和 `pg_restore` 的错误检查。备份文件应写入受权限保护的备份目录，并由运维系统加密；示例不包含真实凭据：

```bash
BACKUP_FILE="/var/backups/alive/alive-$(date +%Y%m%d-%H%M%S).dump"
pg_dump --format=custom --file="$BACKUP_FILE" "$DATABASE_URL"
```

建议至少保留每日备份 14 天、每周备份 8 周，并定期做恢复演练。备份成功不等于可恢复，恢复演练的结果应记录日期、schema version、表计数和错误输出。

## 隔离恢复验证

恢复绝不覆盖开发库或生产库。以下命令使用临时数据库名；完成后只删除这个临时数据库：

```bash
RESTORE_DB="alive_restore_check_YYYYMMDD"
pg_dump --format=custom --file="/tmp/alive-restore-check.dump" "$DATABASE_URL"
createdb -O alive "$RESTORE_DB"
pg_restore --exit-on-error --dbname="postgres://alive:alive@127.0.0.1:5432/$RESTORE_DB?sslmode=disable" /tmp/alive-restore-check.dump
psql "postgres://alive:alive@127.0.0.1:5432/$RESTORE_DB?sslmode=disable" \
  -c 'select version from schema_migrations;'
psql "postgres://alive:alive@127.0.0.1:5432/$RESTORE_DB?sslmode=disable" \
  -c 'select count(*) as entries from entries where deleted_at is null;'
dropdb "$RESTORE_DB"
```

恢复后的 schema version 应与源库一致；如果备份来自发布前版本，先记录版本差异，再在隔离库执行 `migrate -path backend/migrations -database ... up`，最后重新检查版本和关键表。不要对开发库运行集成测试，因为测试会 `TRUNCATE users CASCADE`。

## 发布 smoke test

1. `curl -fsS https://example.com/health` 返回 200；`/health/ready` 返回 200。
2. 未登录访问 `https://example.com/`、一个 `public` 文章链接和一个已知 `unlisted` slug；三者都不要求访客登录。
3. 登录 `/admin/login`，创建或编辑临时草稿，等待 autosave，再刷新确认内容；发布后打开对应 public slug。
4. 确认已知 `private` slug 与不存在 slug 都返回同一类 404，且 private 不进入公开列表；`unlisted` 不进入列表但凭 slug 可读。
5. 请求不存在 API 路径，确认返回 JSON error envelope；确认 desktop 与 375px 宽度没有横向溢出。
6. 发布完成后立即保留 API JSON 日志、migration version、备份文件校验信息和上述 URL/状态码，便于回滚判断。
