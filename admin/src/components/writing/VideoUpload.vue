<script setup lang="ts">
import { ref } from 'vue'
import { mediaApi } from '../../api'
import { uploadMedia } from '../../media'

const props = defineProps<{
  entryId: number | null
  revision: number
  disabled?: boolean
  ensureEntry?: () => Promise<number | null>
}>()
const emit = defineEmits<{ revision: [number]; uploaded: [string] }>()
const progress = ref<number | null>(null)
const error = ref<string | null>(null)

async function choose(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  error.value = null
  progress.value = 0
  try {
    const entryId = props.entryId ?? await props.ensureEntry?.() ?? null
    if (entryId === null) throw new Error('影像草稿创建失败')
    const uploaded = await uploadMedia(file, { entryId, onProgress: (value) => { progress.value = value } })
    const result = await mediaApi.setPrimaryVideo(entryId, { media_id: uploaded.id, revision: props.revision })
    emit('revision', result.revision)
    emit('uploaded', uploaded.url)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '视频上传失败'
  } finally {
    progress.value = null
    input.value = ''
  }
}
</script>

<template>
  <div class="video-upload">
    <label class="video-upload__picker" for="video-upload-input" data-video-picker>
      <span class="video-upload__title">选择视频</span>
      <span class="video-upload__hint">点击此处上传 MP4</span>
      <input id="video-upload-input" class="video-upload__input" type="file" accept="video/mp4" :disabled="disabled || progress !== null" @change="choose" />
    </label>
    <progress v-if="progress !== null" :value="progress" max="100" aria-label="视频上传进度">{{ progress }}%</progress>
    <p v-if="error" class="video-upload__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.video-upload { display: grid; width: 100%; height: 100%; gap: var(--space-2); }
.video-upload__picker { position: relative; display: grid; min-height: 7rem; place-items: center; align-content: center; gap: .35rem; width: 100%; height: 100%; cursor: pointer; color: rgba(255, 255, 255, 0.9); text-align: center; }
.video-upload__input { position: absolute; inset: 0; width: 100%; height: 100%; opacity: 0; cursor: pointer; }
.video-upload__input:disabled { cursor: not-allowed; }
.video-upload__title { font-size: 1rem; font-weight: 600; }
.video-upload__hint { color: rgba(255, 255, 255, 0.64); font-size: .8125rem; }
.video-upload progress { width: 100%; }
.video-upload__error { color: var(--c-danger); font-size: .8125rem; }
</style>
