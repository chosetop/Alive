<script setup lang="ts">
import { onBeforeUnmount, ref } from "vue";
import {
  musicError,
  musicFileMime,
  uploadMusicFile,
  type MusicAsset,
} from "../../api/music";

const props = defineProps<{
  kind: MusicAsset["kind"];
  label: string;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  uploaded: [MusicAsset];
  busy: [boolean];
  pending: [boolean];
}>();
const file = ref<File | null>(null);
const progress = ref(0);
const busy = ref(false);
const error = ref("");
const complete = ref(false);
let controller: AbortController | undefined;
onBeforeUnmount(() => controller?.abort());

async function upload() {
  if (!file.value || busy.value) return;
  busy.value = true;
  complete.value = false;
  error.value = "";
  progress.value = 0;
  emit("busy", true);
  controller = new AbortController();
  try {
    const asset = await uploadMusicFile(
      file.value,
      props.kind,
      (value) => {
        progress.value = value;
      },
      controller.signal,
    );
    if (controller.signal.aborted) return;
    complete.value = true;
    emit("uploaded", asset);
    emit("pending", false);
  } catch (cause) {
    if (!controller.signal.aborted) error.value = musicError(cause);
  } finally {
    busy.value = false;
    emit("busy", false);
  }
}
function select(event: Event) {
  const input = event.target as HTMLInputElement;
  const selected = input.files?.[0];
  input.value = "";
  if (!selected) return;
  try {
    musicFileMime(selected, props.kind);
  } catch (cause) {
    error.value = musicError(cause);
    return;
  }
  file.value = selected;
  emit("pending", true);
  void upload();
}
function discard() {
  file.value = null;
  error.value = "";
  complete.value = false;
  emit("pending", false);
}
</script>

<template>
  <div class="upload">
    <label class="file-label">
      <span>{{ label }}</span>
      <input
        type="file"
        :aria-label="label"
        :disabled="disabled || busy"
        :accept="
          kind === 'audio'
            ? '.mp3,.m4a,.ogg,.wav'
            : '.jpg,.jpeg,.png,.webp,.gif'
        "
        @change="select"
      />
    </label>
    <p class="hint">
      {{
        kind === "audio"
          ? "MP3 / M4A / OGG / WAV，最大 100 MB"
          : "JPG / PNG / WebP / GIF，最大 20 MB"
      }}
    </p>
    <div v-if="file" class="upload-state" role="status" aria-live="polite">
      <span class="filename">{{ file.name }}</span>
      <span>{{
        busy
          ? progress >= 95
            ? "正在确认文件…"
            : `正在上传 ${progress}%`
          : complete
            ? "上传完成"
            : "尚未上传成功"
      }}</span>
      <progress
        v-if="busy"
        max="100"
        :value="progress"
        :aria-label="`${label}进度`"
      />
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div v-if="error && file && !complete" class="actions">
      <button type="button" :disabled="disabled || busy" @click="upload">
        重试上传
      </button>
      <button type="button" :disabled="disabled || busy" @click="discard">
        放弃此文件
      </button>
    </div>
  </div>
</template>
<style scoped src="./music.css"></style>
<style scoped>
.upload {
  padding: var(--space-3);
  border: 1px dashed var(--c-line-strong);
  border-radius: 12px;
}
.file-label {
  display: grid;
  gap: var(--space-2);
  font-weight: 600;
}
input[type="file"] {
  max-width: 100%;
  padding: 0;
  border: 0;
  font-size: 0.8125rem;
}
.upload-state {
  display: grid;
  gap: 0.4rem;
  font-size: 0.8125rem;
  margin-top: 0.75rem;
}
.filename {
  overflow-wrap: anywhere;
  color: var(--c-ink-muted);
}
progress {
  width: 100%;
  accent-color: var(--c-accent);
}
</style>
