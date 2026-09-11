<script setup lang="ts">
import { ref, watch } from 'vue'
import { mediaApi } from '../../api'
import { uploadMedia } from '../../media'

const props = defineProps<{
  entryId: number | null
  revision: number
  disabled?: boolean
  ensureEntry?: () => Promise<number | null>
}>()
const emit = defineEmits<{ revision: [number]; uploaded: [string]; aspect: [number] }>()
const progress = ref<number | null>(null)
const error = ref<string | null>(null)
const videoUrl = ref('')

watch(
  () => props.entryId,
  async (entryId) => {
    videoUrl.value = ''
    if (entryId === null) return
    try {
      const media = await mediaApi.listEntryMedia(entryId)
      videoUrl.value = media.find((item) => item.mime_type === 'video/mp4')?.url ?? ''
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : '已上传的视频读取失败'
    }
  },
  { immediate: true },
)

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
    videoUrl.value = uploaded.url
    emit('revision', result.revision)
    emit('uploaded', uploaded.url)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '视频上传失败'
  } finally {
    progress.value = null
    input.value = ''
  }
}

function reportAspect(event: Event): void {
  const video = event.currentTarget as HTMLVideoElement
  if (video.videoWidth > 0 && video.videoHeight > 0) emit('aspect', video.videoWidth / video.videoHeight)
}
</script>

<template>
  <div class="video-upload">
    <video v-if="videoUrl" class="video-upload__preview" :src="videoUrl" controls playsinline preload="metadata" data-video-preview @loadedmetadata="reportAspect" />
    <label class="video-upload__picker" for="video-upload-input" data-video-picker>
      <span class="video-upload__title">{{ videoUrl ? '更换视频' : '选择视频' }}</span>
      <span class="video-upload__hint">{{ videoUrl ? '点击选择另一段 MP4' : '点击此处上传 MP4' }}</span>
      <input id="video-upload-input" class="video-upload__input" type="file" accept="video/mp4" :disabled="disabled || progress !== null" @change="choose" />
    </label>
    <progress v-if="progress !== null" :value="progress" max="100" aria-label="视频上传进度">{{ progress }}%</progress>
    <p v-if="error" class="video-upload__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.video-upload { position: relative; display: grid; width: 100%; height: 100%; gap: var(--space-2); overflow: hidden; border-radius: inherit; }
.video-upload__preview { display: block; width: 100%; height: 100%; object-fit: contain; background: #05070a; }
.video-upload__picker { position: absolute; inset: auto var(--space-3) var(--space-3); display: grid; min-height: 3.25rem; place-items: center; align-content: center; gap: .15rem; cursor: pointer; color: rgba(255, 255, 255, 0.92); text-align: center; border: 1px solid rgba(255, 255, 255, 0.16); border-radius: .75rem; background: rgba(8, 12, 17, 0.78); backdrop-filter: blur(12px); }
.video-upload__picker:only-child { inset: 0; min-height: 7rem; border: 0; background: transparent; backdrop-filter: none; }
.video-upload__input { position: absolute; inset: 0; width: 100%; height: 100%; opacity: 0; cursor: pointer; }
.video-upload__input:disabled { cursor: not-allowed; }
.video-upload__title { font-size: 1rem; font-weight: 600; }
.video-upload__hint { color: rgba(255, 255, 255, 0.64); font-size: .8125rem; }
.video-upload progress { position: absolute; inset: auto var(--space-3) var(--space-2); width: calc(100% - var(--space-6)); }
.video-upload__error { color: var(--c-danger); font-size: .8125rem; }
</style>
