# 网站背景音乐

后台「音乐」分为曲库和歌单。

1. 在曲库添加歌曲，上传音频，填写名称和创作者，可上传歌曲封面。支持 MP3、M4A、OGG、WAV，单文件不超过 100 MiB；封面支持 JPG、PNG、WebP、GIF，不超过 20 MiB。
2. 上传后可试听；试听加载元数据时会自动填入时长，也可以手动填写。上传失败时可以重试或放弃文件。
3. 新建歌单，从曲库添加歌曲，用「上移」「下移」排序。歌单可设置独立封面、公开状态和默认状态；设为默认会自动公开并替换旧的默认歌单。
4. 前台只显示公开且非空的歌单。访客点击播放后，站内路由切换持续播放；刷新后恢复歌曲、进度、音量和模式，等待访客点击再播放。
5. 曲库和歌单的删除入口位于「管理」模式，并要求确认。删除歌曲会同时移出所有歌单；删除歌单保留曲库歌曲。删除管理记录不会删除 OSS 原始文件。

## 部署

先执行数据库迁移 `000016_create_music`，再启动包含音乐服务的新后端，重新构建前台与后台。音乐接口复用现有 OSS 配置和签名上传客户端；禁用 OSS 时，曲库读取仍可用，上传返回 503。

浏览器音频/封面直接 PUT 至 OSS。Bucket CORS 需要允许实际后台来源、PUT 和 Content-Type；增加开发端口或域名后须更新相应来源。公开状态控制歌单接口可见性，不是文件访问权限：具有 OSS 文件地址的人仍可直接访问文件。第一版不做音频转码和歌词。

## 接口与一致性

- `GET /api/v1/music`：公开且非空的歌单与其引用歌曲；默认歌单优先。
- `GET /api/v1/admin/music`：当前站点所有者的曲库与歌单。
- `POST /api/v1/admin/music/uploads/presign`、`POST /api/v1/admin/music/uploads`：签名与注册上传。注册校验所有者对象路径、文件类型、大小，并通过 OSS Head 校验实际元数据。
- `POST /api/v1/admin/music/tracks`、`PUT/DELETE /api/v1/admin/music/tracks/:id`：歌曲管理。
- `POST /api/v1/admin/music/playlists`、`PUT/DELETE /api/v1/admin/music/playlists/:id`：歌单与有序歌曲关联管理。

PUT 带 `revision`，DELETE 通过查询参数传 `revision`；旧版本返回 409。编辑已有记录时省略 `cover_asset_id` 保留封面，传 `null` 移除封面。数据库事务保证歌单替换、默认切换、歌曲删除与引用修订原子完成。后台接口均需要现有所有者会话。

## 验证

后端：`make check`；音乐数据库集成测试遵循现有 `TEST_DATABASE_URL` 规则，只允许 `_test` 数据库。
后台：`npm run build`、`npm run test:run`。
前台：`npm run typecheck`、`npm run build`、`npx vitest run`。

浏览器验收：实际上传音频和封面、试听、保存和编辑歌单、切换页面不中断播放、播放结束自动下一首、刷新后不自动播放，以及桌面和 375px 下的展开/收起、键盘和长文本布局。
