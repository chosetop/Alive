<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import {
  musicApi,
  musicError,
  type MusicCatalog,
  type MusicPlaylist,
  type MusicTrack,
} from "../api/music";
import { isApiClientError } from "../api/errors";
import { UiDialog } from "../components/ui";
import TrackEditor from "../components/music/TrackEditor.vue";
import PlaylistEditor from "../components/music/PlaylistEditor.vue";

const catalog = ref<MusicCatalog>({ tracks: [], playlists: [] });
const tab = ref<"library" | "playlists">("library");
const loading = ref(true);
const loadError = ref("");
const notice = ref("");
const query = ref("");
const managing = ref(false);
const trackEditor = ref<MusicTrack | "new" | null>(null);
const playlistEditor = ref<MusicPlaylist | "new" | null>(null);
const editing = computed(
  () => trackEditor.value !== null || playlistEditor.value !== null,
);
const preview = ref<MusicTrack | null>(null);
const previewError = ref("");
const deletion = ref<
  | { kind: "track"; item: MusicTrack }
  | { kind: "playlist"; item: MusicPlaylist }
  | null
>(null);
const deleting = ref(false);
const deleteError = ref("");
const deleteConflict = ref(false);
const tracks = computed(() =>
  catalog.value.tracks.filter((track) =>
    `${track.title} ${track.artist}`
      .toLocaleLowerCase()
      .includes(query.value.toLocaleLowerCase()),
  ),
);
const playlists = computed(() =>
  [...catalog.value.playlists].sort(
    (a, b) => Number(b.is_default) - Number(a.is_default) || a.id - b.id,
  ),
);
const deleteTitle = computed(() =>
  deletion.value?.kind === "track"
    ? deletion.value.item.title
    : (deletion.value?.item.name ?? ""),
);
const references = computed(() =>
  deletion.value?.kind === "track"
    ? catalog.value.playlists.filter((list) =>
        list.track_ids.includes(deletion.value!.item.id),
      ).length
    : 0,
);
async function load() {
  if (deleting.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    const data = await musicApi.catalog();
    catalog.value = {
      tracks: data.tracks ?? [],
      playlists: (data.playlists ?? []).map((list) => ({
        ...list,
        track_ids: list.track_ids ?? [],
      })),
    };
  } catch (cause) {
    loadError.value = musicError(cause);
  } finally {
    loading.value = false;
  }
}
onMounted(() => void load());
function closeEditors() {
  trackEditor.value = null;
  playlistEditor.value = null;
}
async function saved() {
  closeEditors();
  notice.value = "已保存。";
  await load();
}
async function reload() {
  closeEditors();
  deletion.value = null;
  preview.value = null;
  await load();
}
function switchTab(value: typeof tab.value) {
  tab.value = value;
  managing.value = false;
  preview.value = null;
  notice.value = "";
}
function editTrack(track: MusicTrack | "new") {
  preview.value = null;
  notice.value = "";
  trackEditor.value = track;
}
function confirmTrack(track: MusicTrack) {
  deletion.value = { kind: "track", item: track };
  deleteError.value = "";
  deleteConflict.value = false;
}
function confirmPlaylist(playlist: MusicPlaylist) {
  deletion.value = { kind: "playlist", item: playlist };
  deleteError.value = "";
  deleteConflict.value = false;
}
async function remove() {
  if (!deletion.value || deleting.value || deleteConflict.value) return;
  deleting.value = true;
  deleteError.value = "";
  try {
    const target = deletion.value;
    if (target.kind === "track")
      await musicApi.deleteTrack(target.item.id, target.item.revision);
    else await musicApi.deletePlaylist(target.item.id, target.item.revision);
    deletion.value = null;
    preview.value = null;
    notice.value = "已删除。";
  } catch (cause) {
    deleteError.value = musicError(cause);
    deleteConflict.value =
      isApiClientError(cause) && (cause.status === 409 || cause.status === 404);
  } finally {
    deleting.value = false;
  }
  if (!deletion.value) await load();
}
function formatDuration(seconds: number) {
  if (!seconds) return "时长未知";
  return `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, "0")}`;
}
</script>
<template>
  <div class="page music">
    <header class="page-heading">
      <div>
        <h1>音乐</h1>
        <p class="hint">整理声音，为来访的人留一份歌单。</p>
      </div>
      <span class="collection-count"
        >{{ catalog.tracks.length }} 首歌曲 ·
        {{ catalog.playlists.length }} 张歌单</span
      >
    </header>
    <div class="tab-bar" role="group" aria-label="音乐管理分区">
      <button
        :aria-pressed="tab === 'library'"
        :disabled="editing"
        @click="switchTab('library')"
      >
        曲库
      </button>
      <button
        :aria-pressed="tab === 'playlists'"
        :disabled="editing"
        @click="switchTab('playlists')"
      >
        歌单
      </button>
    </div>
    <p v-if="notice" class="hint" role="status">{{ notice }}</p>
    <div v-if="loading" class="empty" role="status" aria-live="polite">
      正在整理音乐…
    </div>
    <div v-else-if="loadError" class="notice">
      <p class="error" role="alert">{{ loadError }}</p>
      <button @click="load">重新加载</button>
    </div>
    <TrackEditor
      v-else-if="trackEditor"
      :track="trackEditor === 'new' ? undefined : trackEditor"
      @saved="saved"
      @cancel="closeEditors"
      @reload="reload"
    />
    <PlaylistEditor
      v-else-if="playlistEditor"
      :playlist="playlistEditor === 'new' ? undefined : playlistEditor"
      :tracks="catalog.tracks"
      @saved="saved"
      @cancel="closeEditors"
      @reload="reload"
    />
    <section v-else-if="tab === 'library'" aria-label="曲库">
      <div class="toolbar">
        <label class="search"
          ><span class="sr-only">搜索歌曲或歌手</span
          ><input
            v-model="query"
            type="search"
            aria-label="搜索歌曲或歌手"
            placeholder="搜索歌曲或歌手"
        /></label>
        <div class="actions">
          <button
            v-if="catalog.tracks.length"
            :aria-pressed="managing"
            @click="managing = !managing"
          >
            {{ managing ? "完成管理" : "管理" }}</button
          ><button class="primary" @click="editTrack('new')">添加歌曲</button>
        </div>
      </div>
      <div v-if="!catalog.tracks.length" class="empty">
        <span class="empty-art" aria-hidden="true">♪</span>
        <h2>曲库还是空的</h2>
        <p class="hint">上传第一首歌，声音就有了安放之处。</p>
      </div>
      <p v-else-if="!tracks.length" class="empty">没有找到匹配的歌曲。</p>
      <ul v-else class="track-list">
        <li
          v-for="track in tracks"
          :key="track.id"
          class="track-row"
          :data-track="track.id"
        >
          <img
            v-if="track.cover_url"
            class="cover"
            :src="track.cover_url"
            alt=""
            loading="lazy"
          />
          <span v-else class="cover fallback" aria-hidden="true">♪</span>
          <div class="track-info">
            <h3>{{ track.title }}</h3>
            <p class="hint">
              {{ track.artist || "未知歌手" }}
              <span class="duration"
                >· {{ formatDuration(track.duration) }}</span
              >
            </p>
          </div>
          <div class="actions row-actions">
            <button
              :aria-label="`试听 ${track.title}`"
              @click="
                preview = track;
                previewError = '';
              "
            >
              试听</button
            ><button
              :aria-label="`编辑 ${track.title}`"
              @click="editTrack(track)"
            >
              编辑</button
            ><button
              v-if="managing"
              class="danger"
              :aria-label="`删除 ${track.title}`"
              @click="confirmTrack(track)"
            >
              删除
            </button>
          </div>
        </li>
      </ul>
      <div v-if="preview" class="preview notice">
        <div class="editor-head">
          <h3>试听 · {{ preview.title }}</h3>
          <button aria-label="关闭试听" @click="preview = null">关闭</button>
        </div>
        <audio
          :key="preview.id"
          :src="preview.audio_url"
          controls
          preload="none"
          :aria-label="`试听 ${preview.title}`"
          @error="
            previewError = '暂时无法播放这首歌，请检查网络或替换音频文件。'
          "
        />
        <p v-if="previewError" class="error" role="alert">{{ previewError }}</p>
      </div>
    </section>
    <section v-else aria-label="歌单">
      <div class="toolbar">
        <p class="hint">公开且有歌曲的歌单会出现在前台。</p>
        <div class="actions">
          <button
            v-if="playlists.length"
            :aria-pressed="managing"
            @click="managing = !managing"
          >
            {{ managing ? "完成管理" : "管理" }}</button
          ><button
            class="primary"
            @click="
              playlistEditor = 'new';
              notice = '';
            "
          >
            新建歌单
          </button>
        </div>
      </div>
      <div v-if="!playlists.length" class="empty">
        <span class="empty-art" aria-hidden="true">♫</span>
        <h2>为喜欢的声音编一张歌单</h2>
        <p class="hint">选择歌曲、排好顺序，再决定是否公开。</p>
      </div>
      <ul v-else class="playlist-grid">
        <li
          v-for="playlist in playlists"
          :key="playlist.id"
          class="playlist"
          :data-playlist="playlist.id"
        >
          <img
            v-if="playlist.cover_url"
            class="playlist-art"
            :src="playlist.cover_url"
            alt=""
            loading="lazy"
          /><span v-else class="playlist-art fallback" aria-hidden="true"
            >♫</span
          >
          <div class="playlist-body">
            <div class="badges">
              <span v-if="playlist.is_default" class="badge default">默认</span
              ><span class="badge">{{
                playlist.is_public ? "公开" : "私密"
              }}</span
              ><span class="hint">{{ playlist.track_ids.length }} 首</span>
            </div>
            <h2>{{ playlist.name }}</h2>
            <p v-if="!playlist.track_ids.length" class="hint">
              添加歌曲后才会在前台显示
            </p>
            <div class="actions">
              <button
                :aria-label="`编辑歌单 ${playlist.name}`"
                @click="
                  playlistEditor = playlist;
                  notice = '';
                "
              >
                编辑歌单</button
              ><button
                v-if="managing"
                class="danger"
                :aria-label="`删除歌单 ${playlist.name}`"
                @click="confirmPlaylist(playlist)"
              >
                删除
              </button>
            </div>
          </div>
        </li>
      </ul>
    </section>
    <UiDialog
      :open="!!deletion"
      :title="`删除「${deleteTitle}」？`"
      :description="
        deletion?.kind === 'track'
          ? `歌曲将从曲库及 ${references} 张歌单中移除，存储中的原始文件会保留。`
          : '只删除这张歌单，曲库中的歌曲仍会保留。'
      "
      @update:open="
        (open) => {
          if (!open && !deleting) deletion = null;
        }
      "
    >
      <p class="hint">此操作无法撤销。</p>
      <p v-if="deleteError" class="error" role="alert">{{ deleteError }}</p>
      <template #actions
        ><button :disabled="deleting" @click="deletion = null">取消</button
        ><button v-if="deleteConflict" @click="reload">重新载入列表</button
        ><button v-else class="danger" :disabled="deleting" @click="remove">
          {{ deleting ? "正在删除…" : "确认删除" }}
        </button></template
      >
    </UiDialog>
  </div>
</template>
<style scoped src="../components/music/music.css"></style>
<style scoped>
.page {
  max-width: 68rem;
  display: grid;
  gap: 1.5rem;
}
.page-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
}
.page-heading .hint {
  margin-top: 0.5rem;
}
.collection-count {
  font-size: 0.75rem;
  color: var(--c-ink-faint);
  white-space: nowrap;
}
.tab-bar {
  display: flex;
  width: fit-content;
  padding: 0.25rem;
  gap: 0.25rem;
  border-radius: 16px;
  background: var(--c-surface-sunken);
}
.tab-bar button {
  border: 0;
  background: transparent;
  min-width: 5rem;
}
.tab-bar button[aria-pressed="true"] {
  background: var(--c-surface);
  font-weight: 600;
  box-shadow: 0 1px 3px rgb(0 0 0 / 0.08);
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.25rem;
}
.search {
  width: min(100%, 21rem);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.track-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.track-row {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem 0;
  border-bottom: 1px solid var(--c-line);
}
.track-info {
  min-width: 0;
  flex: 1;
  overflow-wrap: anywhere;
}
.duration {
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--c-ink-faint);
  font-size: 1.75rem;
  background: var(--c-surface-sunken);
}
.preview {
  margin-top: 1.5rem;
  display: grid;
  gap: 0.75rem;
}
.playlist-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
  list-style: none;
  padding: 0;
  margin: 0;
}
.playlist {
  border: 1px solid var(--c-line);
  border-radius: 16px;
  overflow: hidden;
  background: var(--c-surface);
  min-width: 0;
}
.playlist-art {
  width: 100%;
  aspect-ratio: 2 / 1;
  object-fit: cover;
}
.playlist-art.fallback {
  font-size: 3rem;
}
.playlist-body {
  display: grid;
  gap: 0.85rem;
  padding: 1.25rem;
  overflow-wrap: anywhere;
}
.badges {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.badge {
  font-size: 0.6875rem;
  border: 1px solid var(--c-line);
  border-radius: 6px;
  padding: 0.15rem 0.4rem;
  color: var(--c-ink-muted);
}
.badge.default {
  color: var(--c-accent);
  border-color: var(--c-accent);
}
.empty-art {
  font-size: 3rem;
  color: var(--c-ink-faint);
}
@media (max-width: 48rem) {
  .page-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .track-row {
    flex-wrap: wrap;
  }
  .row-actions {
    width: 100%;
    padding-left: 5rem;
  }
}
@media (max-width: 40rem) {
  .toolbar {
    align-items: stretch;
    flex-direction: column;
  }
  .toolbar > .actions {
    justify-content: flex-end;
  }
  .playlist-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .row-actions {
    padding-left: 0;
  }
}
</style>
