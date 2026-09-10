<script setup lang="ts">
import { computed, ref } from "vue";
import {
  musicApi,
  musicError,
  type MusicAsset,
  type MusicTrack,
  type TrackInput,
} from "../../api/music";
import { isApiClientError } from "../../api/errors";
import MusicUpload from "./MusicUpload.vue";

const props = defineProps<{ track?: MusicTrack }>();
const emit = defineEmits<{ saved: []; cancel: []; reload: [] }>();
const title = ref(props.track?.title ?? "");
const artist = ref(props.track?.artist ?? "");
const duration = ref(props.track?.duration ?? 0);
const audioAsset = ref<MusicAsset>();
const coverAssetId = ref<number | null | undefined>(
  props.track ? undefined : null,
);
const coverUrl = ref(props.track?.cover_url ?? "");
const audioBusy = ref(false);
const coverBusy = ref(false);
const audioPending = ref(false);
const coverPending = ref(false);
const saving = ref(false);
const error = ref("");
const conflict = ref(false);
const busy = computed(() => saving.value || audioBusy.value || coverBusy.value);
const previewUrl = computed(
  () => audioAsset.value?.url ?? props.track?.audio_url,
);
function setCover(asset: MusicAsset) {
  coverAssetId.value = asset.id;
  coverUrl.value = asset.url;
}
function setAudio(asset: MusicAsset) {
  audioAsset.value = asset;
  duration.value = 0;
}
function metadata(event: Event) {
  const value = (event.target as HTMLAudioElement).duration;
  if (
    audioAsset.value &&
    Number.isFinite(value) &&
    value >= 0 &&
    value <= 86400
  )
    duration.value = value;
}
async function save() {
  if (busy.value || audioPending.value || coverPending.value || conflict.value)
    return;
  if (
    !title.value.trim() ||
    title.value.trim().length > 200 ||
    artist.value.trim().length > 200
  ) {
    error.value = "请填写歌曲名称，名称和歌手均不能超过 200 字。";
    return;
  }
  if (!props.track && !audioAsset.value) {
    error.value = "请先上传一首音频。";
    return;
  }
  if (
    !Number.isFinite(duration.value) ||
    duration.value < 0 ||
    duration.value > 86400
  ) {
    error.value = "时长应在 0 至 86400 秒之间。";
    return;
  }
  saving.value = true;
  error.value = "";
  const body: TrackInput = {
    title: title.value.trim(),
    artist: artist.value.trim(),
    duration: duration.value,
    ...(audioAsset.value ? { audio_asset_id: audioAsset.value.id } : {}),
    ...(coverAssetId.value !== undefined
      ? { cover_asset_id: coverAssetId.value }
      : {}),
  };
  try {
    if (props.track)
      await musicApi.updateTrack(props.track.id, {
        ...body,
        revision: props.track.revision,
      });
    else
      await musicApi.createTrack({
        ...body,
        audio_asset_id: audioAsset.value!.id,
      });
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
    aria-label="歌曲编辑"
    :aria-busy="busy"
    @submit.prevent="save"
  >
    <div class="editor-head">
      <h2>{{ track ? "编辑歌曲" : "添加歌曲" }}</h2>
      <button type="button" :disabled="busy" @click="emit('cancel')">
        取消
      </button>
    </div>
    <div class="fields">
      <label class="field"
        >歌曲名称<input
          v-model="title"
          aria-label="歌曲名称"
          maxlength="200"
          required
          :disabled="saving"
      /></label>
      <label class="field"
        >歌手 / 创作者<input
          v-model="artist"
          aria-label="歌手 / 创作者"
          maxlength="200"
          placeholder="可留空"
          :disabled="saving"
      /></label>
    </div>
    <MusicUpload
      kind="audio"
      :label="track ? '替换音频' : '上传音频'"
      :disabled="saving"
      @uploaded="setAudio"
      @busy="audioBusy = $event"
      @pending="audioPending = $event"
    />
    <div v-if="previewUrl" class="field">
      <span>音频试听</span>
      <audio
        :key="previewUrl"
        :src="previewUrl"
        controls
        preload="none"
        aria-label="音频试听"
        @loadedmetadata="metadata"
        @error="error = '音频暂时无法播放，请检查文件或重新上传。'"
      />
    </div>
    <label class="field"
      >时长（秒）<input
        v-model.number="duration"
        type="number"
        aria-label="时长（秒）"
        min="0"
        max="86400"
        step="any"
        required
        :disabled="saving"
      /><span class="hint"
        >试听后自动读取新音频时长，也可手动填写；未知时可填 0。</span
      ></label
    >
    <MusicUpload
      kind="image"
      label="上传歌曲封面"
      :disabled="saving"
      @uploaded="setCover"
      @busy="coverBusy = $event"
      @pending="coverPending = $event"
    />
    <div v-if="coverUrl" class="cover-preview">
      <img :src="coverUrl" alt="歌曲封面预览" /><button
        type="button"
        :disabled="busy || coverPending"
        @click="
          coverUrl = '';
          coverAssetId = null;
        "
      >
        移除封面
      </button>
    </div>
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
        :disabled="busy || audioPending || coverPending || conflict"
      >
        {{ saving ? "正在保存…" : "保存歌曲" }}</button
      ><span class="hint">{{
        audioPending || coverPending
          ? "请完成上传或放弃失败文件后保存。"
          : "加入曲库后，可在歌单中选择这首歌。"
      }}</span>
    </div>
  </form>
</template>
<style scoped src="./music.css"></style>
