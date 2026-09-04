<script setup lang="ts">
import MarkdownEditor from '../MarkdownEditor.vue'

const props = withDefaults(
  defineProps<{
    title: string
    content: string
    disabled?: boolean
  }>(),
  { disabled: false },
)

const emit = defineEmits<{
  'update:title': [string]
  'update:content': [string]
}>()
</script>

<template>
    <section class="journal-canvas" data-journal-canvas>
    <div class="journal-canvas__frame">
      <div class="journal-canvas__page">
        <span class="journal-canvas__edge" aria-hidden="true" />
        <div class="journal-canvas__body">
          <input
            :value="props.title"
            :disabled="props.disabled"
            class="journal-canvas__title"
            data-journal-title
            aria-label="日志标题"
            placeholder="写下这篇日志的标题"
            @input="emit('update:title', ($event.target as HTMLInputElement).value)"
          />
          <MarkdownEditor
            :initial-value="props.content"
            :disabled="props.disabled"
            @update="emit('update:content', $event)"
          />
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.journal-canvas {
  min-width: 0;
}

.journal-canvas__frame {
  display: grid;
  gap: var(--space-4);
}

.journal-canvas__meta {
  display: grid;
  gap: var(--space-1);
}

.journal-canvas__eyebrow {
  margin: 0;
  color: var(--c-ink-muted);
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.journal-canvas__context {
  margin: 0;
  color: var(--c-ink-faint);
  font-size: 0.875rem;
}

.journal-canvas__page {
  position: relative;
  padding: clamp(var(--space-4), 5vw, var(--space-6));
  border: 1px solid var(--c-line);
  border-radius: 1.125rem;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--c-paper) 96%, var(--c-accent) 4%), var(--c-paper));
  box-shadow: var(--shadow-float);
}

.journal-canvas__edge {
  position: absolute;
  inset-block: var(--space-5);
  inset-inline-start: clamp(var(--space-4), 5vw, var(--space-6));
  width: 1px;
  background: color-mix(in srgb, var(--c-accent) 28%, transparent);
}

.journal-canvas__body {
  min-width: 0;
  padding-inline-start: clamp(var(--space-4), 4vw, var(--space-5));
}

.journal-canvas :deep(.editor-shell) {
  min-height: 38rem;
}

.journal-canvas :deep(.editor-shell .milkdown) {
  max-width: none;
}

.journal-canvas :deep(.editor-shell .ProseMirror) {
  font-size: 1.3rem;
  line-height: 1.95;
}

.journal-canvas :deep(.editor-shell .ProseMirror p) {
  font-size: 1.3rem;
  margin-block: 1.1em;
}

.journal-canvas__title {
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-family: var(--font-heading);
  font-size: clamp(2rem, 5vw, 2.75rem);
  font-weight: 600;
  letter-spacing: -0.03em;
}

.journal-canvas__title::placeholder {
  color: color-mix(in srgb, var(--c-ink-faint) 78%, transparent);
}

.journal-canvas__title:focus {
  outline: none;
}

.journal-canvas__title:disabled {
  color: var(--c-ink-faint);
}

.journal-canvas__date {
  margin: var(--space-3) 0 0;
  color: var(--c-ink-faint);
  font-size: 0.8125rem;
}

@media (max-width: 40rem) {
  .journal-canvas :deep(.editor-shell) {
    min-height: 28rem;
  }
  .journal-canvas__page {
    padding-inline: var(--space-4);
  }

  .journal-canvas__edge {
    inset-inline-start: var(--space-4);
  }

  .journal-canvas__body {
    padding-inline-start: var(--space-4);
  }
}
</style>
