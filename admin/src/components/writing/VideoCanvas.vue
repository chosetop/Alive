<script setup lang="ts">
import type { EntryDetail } from '../../types/api'
import VideoUpload from './VideoUpload.vue'

const props = withDefaults(
  defineProps<{
    entry: EntryDetail | null
    title: string
    summary: string
    disabled?: boolean
  }>(),
  { disabled: false },
)

const emit = defineEmits<{
  'update:title': [string]
  'update:summary': [string]
  revision: [number]
}>()
</script>

<template>
  <section class="video-canvas" data-video-canvas>
    <header class="video-canvas__meta">
      <p class="video-canvas__eyebrow">影像工作台</p>
      <p class="video-canvas__context">先安放画面，再补充标题与说明。</p>
    </header>

    <section class="video-canvas__viewfinder" data-video-viewfinder data-aspect="16:9">
      <div v-if="props.entry" class="video-canvas__upload-shell">
        <VideoUpload
          :entry-id="props.entry.id"
          :revision="props.entry.revision"
          :disabled="props.disabled"
          data-video-upload
          @revision="emit('revision', $event)"
        />
      </div>
      <div v-else class="video-canvas__empty" data-video-empty>
        <strong>主视频会出现在这里</strong>
        <p>首次有意义输入后创建草稿，随后这里会出现选择视频、上传进度和播放预览。</p>
      </div>
    </section>

    <div class="video-canvas__fields">
      <input
        :value="props.title"
        :disabled="props.disabled"
        class="video-canvas__title"
        data-video-title
        aria-label="影像标题"
        placeholder="给这段影像一个标题"
        @input="emit('update:title', ($event.target as HTMLInputElement).value)"
      />
      <textarea
        :value="props.summary"
        :disabled="props.disabled"
        class="video-canvas__summary"
        data-video-summary
        aria-label="影像说明"
        placeholder="写下这一段画面的说明、旁白或拍摄线索"
        @input="emit('update:summary', ($event.target as HTMLTextAreaElement).value)"
      />
    </div>
  </section>
</template>

<style scoped>
.video-canvas {
  display: grid;
  gap: var(--space-4);
  min-width: 0;
}

.video-canvas__meta {
  display: grid;
  gap: var(--space-1);
}

.video-canvas__eyebrow {
  margin: 0;
  color: var(--c-ink-muted);
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.video-canvas__context {
  margin: 0;
  color: var(--c-ink-faint);
  font-size: 0.875rem;
}

.video-canvas__viewfinder {
  position: relative;
  display: grid;
  aspect-ratio: 16 / 9;
  align-items: center;
  padding: clamp(var(--space-4), 5vw, var(--space-5));
  border: 1px solid color-mix(in srgb, var(--c-ink) 10%, transparent);
  border-radius: 1.125rem;
  background:
    linear-gradient(180deg, rgba(10, 16, 22, 0.96), rgba(18, 26, 35, 0.92));
  box-shadow: var(--shadow-float);
  overflow: hidden;
}

.video-canvas__upload-shell,
.video-canvas__empty {
  position: absolute;
  inset: var(--space-4);
  display: grid;
  place-items: center;
  padding: var(--space-4);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: calc(1.125rem - var(--space-3));
  background: rgba(8, 12, 17, 0.3);
}

.video-canvas__empty {
  gap: var(--space-2);
  align-content: center;
  text-align: left;
  color: rgba(255, 255, 255, 0.88);
}

.video-canvas__empty strong {
  font-size: 1rem;
  font-weight: 600;
}

.video-canvas__empty p {
  margin: 0;
  color: rgba(255, 255, 255, 0.72);
  font-size: 0.875rem;
  line-height: 1.7;
}

.video-canvas__fields {
  display: grid;
  gap: var(--space-3);
  padding: clamp(var(--space-4), 4vw, var(--space-5));
  border: 1px solid var(--c-line);
  border-radius: 1rem;
  background: var(--c-paper);
}

.video-canvas__title,
.video-canvas__summary {
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-family: inherit;
}

.video-canvas__title:focus,
.video-canvas__summary:focus {
  outline: none;
}

.video-canvas__title:disabled,
.video-canvas__summary:disabled {
  color: var(--c-ink-faint);
}

.video-canvas__title {
  padding: 0;
  font-family: var(--font-heading);
  font-size: clamp(1.75rem, 4vw, 2.375rem);
  font-weight: 600;
  letter-spacing: -0.03em;
}

.video-canvas__summary {
  min-height: 7.5rem;
  padding: 0;
  resize: vertical;
  color: var(--c-ink-muted);
  font-size: 0.95rem;
  line-height: 1.8;
}

@media (max-width: 40rem) {
  .video-canvas__viewfinder {
    min-height: 14rem;
    padding: var(--space-3);
  }

  .video-canvas__upload-shell,
  .video-canvas__empty {
    inset: var(--space-3);
  }

  .video-canvas__fields {
    padding: var(--space-4);
  }
}
</style>
