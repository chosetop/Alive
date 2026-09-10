<script setup lang="ts">
import { computed, ref } from "vue";
import {
  musicApi,
  musicError,
  type MusicAsset,
  type MusicPlaylist,
  type MusicTrack,
} from "../../api/music";
import { isApiClientError } from "../../api/errors";
import MusicUpload from "./MusicUpload.vue";

const props = defineProps<{ playlist?: MusicPlaylist; tracks: MusicTrack[] }>();
const emit = defineEmits<{ saved: []; cancel: []; reload: [] }>();
const name = ref(props.playlist?.name ?? "");
const isPublic = ref(props.playlist?.is_public ?? false);
const isDefault = ref(props.playlist?.is_default ?? false);
const ids = ref([...(props.playlist?.track_ids ?? [])]);
const coverId = ref<number | null | undefined>(
  props.playlist ? undefined : null,
);
const coverUrl = ref(props.playlist?.cover_url ?? "");
const uploadBusy = ref(false);
const uploadPending = ref(false);
const saving = ref(false);
const error = ref("");
const conflict = ref(false);
const query = ref("");
const busy = computed(() => saving.value || uploadBusy.value);
const available = computed(() =>
  props.tracks.filter(
    (track) =>
      !ids.value.includes(track.id) &&
      `${track.title} ${track.artist}`
        .toLocaleLowerCase()
        .includes(query.value.toLocaleLowerCase()),
  ),
);
const selected = computed(() =>
  ids.value
    .map((id) => props.tracks.find((track) => track.id === id))
    .filter((track): track is MusicTrack => !!track),
);
function move(index: number, offset: number) {
  const next = index + offset;
  if (next < 0 || next >= ids.value.length) return;
  const [id] = ids.value.splice(index, 1);
  if (id !== undefined) ids.value.splice(next, 0, id);
}
function setPublic(event: Event) {
  isPublic.value = (event.target as HTMLInputElement).checked;
  if (!isPublic.value) isDefault.value = false;
}
function setDefault(event: Event) {
  isDefault.value = (event.target as HTMLInputElement).checked;
  if (isDefault.value) isPublic.value = true;
}
function setCover(asset: MusicAsset) {
  coverId.value = asset.id;
  coverUrl.value = asset.url;
}
async function save() {
  if (busy.value || uploadPending.value || conflict.value) return;
  if (!name.value.trim() || name.value.trim().length > 200) {
    error.value = "请填写歌单名称，最多 200 字。";
    return;
  }
  saving.value = true;
  error.value = "";
  const body = {
    name: name.value.trim(),
    is_public: isPublic.value,
    is_default: isDefault.value,
    track_ids: [...ids.value],
    ...(coverId.value !== undefined ? { cover_asset_id: coverId.value } : {}),
  };
  try {
    if (props.playlist)
      await musicApi.updatePlaylist(props.playlist.id, {
        ...body,
        revision: props.playlist.revision,
      });
    else await musicApi.createPlaylist(body);
    emit("saved");
  } catch (cause) {
    error.value = musicError(cause);
    conflict.value =
      isApiClientError(cause) && (cause.status === 409 || cause.status === 404);
  } finally {
    saving.value = false;
  }
}
</script>
<template>
  <form
    class="editor"
    aria-label="歌单编辑"
    :aria-busy="busy"
    @submit.prevent="save"
  >
    <div class="editor-head">
      <h2>{{ playlist ? "编辑歌单" : "新建歌单" }}</h2>
      <button type="button" :disabled="busy" @click="emit('cancel')">
        取消
      </button>
    </div>
    <label class="field"
      >歌单名称<input
        v-model="name"
        aria-label="歌单名称"
        maxlength="200"
        required
        :disabled="saving"
        placeholder="给这一组声音起个名字"
    /></label>
    <MusicUpload
      kind="image"
      label="上传歌单封面"
      :disabled="saving"
      @uploaded="setCover"
      @busy="uploadBusy = $event"
      @pending="uploadPending = $event"
    />
    <div v-if="coverUrl" class="cover-preview">
      <img :src="coverUrl" alt="歌单封面预览" /><button
        type="button"
        :disabled="busy || uploadPending"
        @click="
          coverUrl = '';
          coverId = null;
        "
      >
        移除封面
      </button>
    </div>
    <div class="visibility">
      <label class="check"
        ><input
          type="checkbox"
          aria-label="公开歌单"
          :checked="isPublic"
          :disabled="saving"
          @change="setPublic"
        />公开歌单</label
      >
      <label class="check"
        ><input
          type="checkbox"
          aria-label="设为默认歌单"
          :checked="isDefault"
          :disabled="saving"
          @change="setDefault"
        />设为默认歌单</label
      >
      <p class="hint">
        默认歌单会自动公开，并替换原来的默认歌单。空歌单不会在前台显示。
      </p>
    </div>
    <section class="selection" aria-label="歌单歌曲">
      <div class="selection-head">
        <h3>播放顺序</h3>
        <span class="hint">{{ ids.length }} 首</span>
      </div>
      <p v-if="!ids.length" class="hint">
        从下方曲库选择歌曲，再调整播放顺序。
      </p>
      <ol v-else class="selected-list">
        <li
          v-for="(track, index) in selected"
          :key="track.id"
          :data-selected-track="track.id"
        >
          <span class="number">{{ String(index + 1).padStart(2, "0") }}</span>
          <span class="track-title"
            >{{ track.title
            }}<small>{{ track.artist || "未知歌手" }}</small></span
          >
          <div class="actions order-actions">
            <button
              type="button"
              :aria-label="`上移 ${track.title}`"
              :disabled="saving || index === 0"
              @click="move(index, -1)"
            >
              上移
            </button>
            <button
              type="button"
              :aria-label="`下移 ${track.title}`"
              :disabled="saving || index === ids.length - 1"
              @click="move(index, 1)"
            >
              下移
            </button>
            <button
              type="button"
              :aria-label="`移出 ${track.title}`"
              :disabled="saving"
              @click="ids = ids.filter((id) => id !== track.id)"
            >
              移出
            </button>
          </div>
        </li>
      </ol>
      <label class="field"
        >从曲库添加<input
          v-model="query"
          type="search"
          aria-label="搜索曲库歌曲"
          placeholder="搜索歌曲或歌手"
      /></label>
      <div class="available-list">
        <label v-for="track in available" :key="track.id" class="check"
          ><input
            type="checkbox"
            :aria-label="`添加 ${track.title}`"
            :disabled="saving"
            @change="ids.push(track.id)"
          /><span
            >{{ track.title
            }}<small>{{ track.artist || "未知歌手" }}</small></span
          ></label
        >
        <p v-if="!available.length" class="hint">
          {{
            !tracks.length
              ? "曲库还没有歌曲，请先添加音频。"
              : query
                ? "没有匹配的歌曲。"
                : "曲库中的歌曲已全部加入。"
          }}
        </p>
      </div>
    </section>
    <div v-if="error" class="notice">
      <p class="error" role="alert">{{ error }}</p>
      <button v-if="conflict" type="button" @click="emit('reload')">
        放弃当前修改并重新载入
      </button>
    </div>
    <div class="actions">
      <button
        class="primary"
        type="submit"
        :disabled="busy || uploadPending || conflict"
      >
        {{ saving ? "正在保存…" : "保存歌单" }}</button
      ><span v-if="uploadPending" class="hint"
        >请完成封面上传或放弃失败文件后保存。</span
      >
    </div>
  </form>
</template>
<style scoped src="./music.css"></style>
<style scoped>
.selection {
  display: grid;
  gap: 1rem;
  min-width: 0;
}
.selection-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.selected-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.selected-list li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--c-line);
}
.number {
  font-size: 0.75rem;
  color: var(--c-ink-faint);
  font-variant-numeric: tabular-nums;
}
.track-title {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 0.875rem;
}
small {
  display: block;
  color: var(--c-ink-muted);
  font-size: 0.75rem;
  margin-top: 0.25rem;
}
.available-list {
  max-height: 18rem;
  overflow-y: auto;
  padding: 0.5rem 0.75rem;
  background: var(--c-paper);
  border-radius: 12px;
}
.available-list .check {
  padding: 0.5rem 0;
  overflow-wrap: anywhere;
}
@media (max-width: 40rem) {
  .selected-list li {
    flex-wrap: wrap;
  }
  .order-actions {
    width: 100%;
    padding-left: 1.5rem;
  }
}
</style>
