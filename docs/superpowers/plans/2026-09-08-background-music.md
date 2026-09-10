# Alive background music — approved design and implementation plan

Goal: owner-managed audio library, multiple playlists and a persistent public floating player.
Architecture: Go/Gin + PostgreSQL independent music domain; reuse OSS presigned PUT/Head. Vue admin; Nuxt root singleton audio element. No new dependencies, no lyrics/transcoding, no changes to existing WIP.

API contract (all responses existing {data:...} envelope; DELETE returns 204):
Track: {id:number,title:string,artist:string,audio_url:string,cover_url:string,duration:number,revision:number}
Playlist: {id:number,name:string,cover_url:string,is_public:boolean,is_default:boolean,track_ids:number[],revision:number}
Catalog: {tracks:Track[],playlists:Playlist[]}
GET /admin/music => Catalog; GET /music => public nonempty playlists and only their tracks, default first.
POST /admin/music/uploads/presign body {filename,mime_type,size_bytes} => {object_key,upload:{url,method,headers}}
POST /admin/music/uploads body {object_key,mime_type,size_bytes} => {id:number,url:string,kind:'audio'|'image'}
POST /admin/music/tracks body {title,artist,audio_asset_id:number,cover_asset_id:number|null,duration:number} => Track
PUT /admin/music/tracks/:id same body with revision:number => Track; audio_asset_id optional on update means preserve; cover_asset_id omitted preserve, null clears.
DELETE /admin/music/tracks/:id?revision=N => 204 (remove all playlist references, do not delete OSS blob)
POST /admin/music/playlists body {name,cover_asset_id:number|null,is_public:boolean,is_default:boolean,track_ids:number[]} => Playlist
PUT /admin/music/playlists/:id same body with revision:number => Playlist; cover_asset_id omitted preserve, null clears.
DELETE /admin/music/playlists/:id?revision=N => 204
Supports audio/mpeg (.mp3), audio/mp4 (.m4a), audio/ogg (.ogg), audio/wav (.wav); audio <=100MiB; image jpeg/png/webp/gif <=20MiB. Backend verifies owner prefix, Head mime/size and unique registered key. Text <=200 chars; duration finite 0..86400. Revision mismatch =>409; invalid =>400; missing =>404; disabled OSS =>503. Only one default, must be public; list order default first then id. Deleted playlist keeps library tracks.

## Task 1 backend (controller)
- Add migration 000016 music_assets/music_tracks/music_playlists/music_playlist_tracks, ownership and reference constraints.
- Add internal/music model/repository/service and internal/musichttp handler. Wire through router and cmd/server.
- Test validation, owner upload registration, stale revisions/public visibility/playlist atomicity; run make check and DB integration against *_test only.
## Task 2 admin (worker)
- Add src/api/music.ts, src/views/Music.vue and focused components/tests. Add route /music and navigation in AdminLayout only.
- Music library upload/edit/preview/delete confirmation; playlists create/edit/cover/public/default, select tracks and move up/down order. Progress and manual retry after upload failures. Empty/loading/error/conflict states. Proper Chinese labels, themes, mobile 375px.
- Use existing request(path,{method,body}) client; inspect before use. Preserve all unrelated files.
- Test meaningful management behavior and run npm run build/test:run with Node22.
## Task 3 player (worker)
- Add types/music.ts, components/music/*, composables/useMusicPlayer.ts and pure utilities/tests; integrate in frontend/app.vue outside NuxtPage.
- Load GET /music on client. Default playlist; hide empty; collapsed desktop lower right, mobile bottom, expanded song/playlist picker, cover, transport, seek, volume, cycle list/single/shuffle.
- Single audio instance survives routes; do not autoplay initial/refresh; localStorage track/playlist/position/volume/mode guarded and validated. Race-safe rapid selection; ended/error/metadata handling; accessibility keyboard, escape close and focus restore, reduced motion. Browser media errors actionable. No eager audio download before click.
- Tests restoration, end modes/empty and race state; frontend typecheck/lint/build and rendered verification.
## Task 4 integration and review
- Review all patches, API contract compatibility and privacy/auth; start isolated API/test database and app ports; use synthetic audio and cover only, do not touch real articles or credentials.
- Verify actual UI upload/playlist/player navigation desktop and 375px; distinguish mocked and real OSS proof.
- Copy only task changed files from isolated worktree to user working tree after checking target baseline; leave task changes uncommitted and existing WIP intact.

Design: retain Alive --c-paper/--c-ink/--c-accent and --font-ui; 12px controls,16px surfaces, restrained shadow. Album art supplies visual anchor; CSS scoped, existing theme tokens, no new UI framework. Admin organized library/playlist editor; public widget lightweight, no large promotional header. Interaction: 160ms press feedback and 240ms expand, respects reduced motion.
