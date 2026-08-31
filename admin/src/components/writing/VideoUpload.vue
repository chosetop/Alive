<script setup lang="ts">
import { ref } from 'vue'
import { mediaApi } from '../../api'
import { uploadMedia } from '../../media'

const props = defineProps<{ entryId: number; revision: number; disabled?: boolean }>()
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
    const uploaded = await uploadMedia(file, { entryId: props.entryId, onProgress: (value) => { progress.value = value } })
    const result = await mediaApi.setPrimaryVideo(props.entryId, { media_id: uploaded.id, revision: props.revision })
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
    <label class="field" for="video-upload-input">主视频</label>
    <input id="video-upload-input" type="file" accept="video/mp4" :disabled="disabled || progress !== null" @change="choose" />
    <progress v-if="progress !== null" :value="progress" max="100" aria-label="视频上传进度">{{ progress }}%</progress>
    <p v-if="error" class="video-upload__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.video-upload { display: grid; gap: var(--space-2); margin-top: var(--space-3); padding-top: var(--space-3); border-top: 1px solid var(--c-line); }
.video-upload input, .video-upload progress { width: 100%; }
.video-upload__error { color: var(--c-danger); font-size: .8125rem; }
</style>
