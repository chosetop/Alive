<script setup lang="ts">
import { ref } from 'vue'
import { uploadMedia } from '../../media'
import type { MediaRecord } from '../../api/media'

const props = withDefaults(defineProps<{ entryId: number; disabled?: boolean }>(), { disabled: false })
const emit = defineEmits<{ uploaded: [MediaRecord] }>()
const progress = ref<number | null>(null)
const error = ref<string | null>(null)

async function choose(event: Event): Promise<void> {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  error.value = null; progress.value = 0
  try {
    const media = await uploadMedia(file, { entryId: props.entryId, onProgress: (value) => { progress.value = value } })
    emit('uploaded', media)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '上传失败' }
  finally { progress.value = null; (event.target as HTMLInputElement).value = '' }
}
</script>

<template>
  <div class="media-upload">
    <label class="field" for="media-upload-input">添加图片或视频</label>
    <input id="media-upload-input" type="file" accept="image/jpeg,image/png,image/webp,image/gif,video/mp4" :disabled="disabled || progress !== null" @change="choose" />
    <progress v-if="progress !== null" :value="progress" max="100" aria-label="上传进度">{{ progress }}%</progress>
    <p v-if="error" class="media-upload__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.media-upload { display: grid; gap: var(--space-2); margin-top: var(--space-3); padding-top: var(--space-3); border-top: 1px solid var(--c-line); }
.media-upload input { width: 100%; }
.media-upload progress { width: 100%; }
.media-upload__error { color: var(--c-danger); font-size: .8125rem; }
</style>
