<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'

import { THEMES, type ThemeName } from '@alive/theme'
import { toUserMessage } from '../../api'
import { getSiteSettings } from '../../api/site'
import { useThemeStore } from '../../stores/theme'
import { UiButton, UiIcon } from '../ui'

const theme = useThemeStore()
const saving = ref(false)
const error = ref<string | null>(null)
const open = ref(false)

const currentThemeLabel = computed(
  () => THEMES.find((item) => item.name === theme.preview)?.label ?? '',
)

function selectTheme(name: string): void {
  if (!THEMES.some((item) => item.name === name)) return
  theme.previewTheme(name as ThemeName)
  error.value = null
}

function handleOptionKeydown(event: KeyboardEvent, currentIndex: number): void {
  let nextIndex: number

  switch (event.key) {
    case 'ArrowRight':
    case 'ArrowDown':
      nextIndex = (currentIndex + 1) % THEMES.length
      break
    case 'ArrowLeft':
    case 'ArrowUp':
      nextIndex = (currentIndex - 1 + THEMES.length) % THEMES.length
      break
    case 'Home':
      nextIndex = 0
      break
    case 'End':
      nextIndex = THEMES.length - 1
      break
    default:
      return
  }

  event.preventDefault()
  const group = (event.currentTarget as HTMLElement).closest('[role="radiogroup"]')
  selectTheme(THEMES[nextIndex].name)
  void nextTick(() => group?.querySelectorAll<HTMLElement>('[role="radio"]')[nextIndex]?.focus())
}

async function save(): Promise<void> {
  if (!theme.isDirty || saving.value) return
  saving.value = true
  error.value = null
  try {
    await theme.saveDefault()
  } catch (saveFailure) {
    if ((saveFailure as { status?: number }).status === 409) {
      try {
        const desired = theme.preview
        theme.hydrate(await getSiteSettings())
        theme.previewTheme(desired)
        await theme.saveDefault()
        return
      } catch (retryFailure) {
        error.value = toUserMessage(retryFailure)
      }
    } else error.value = toUserMessage(saveFailure)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="theme-picker" data-theme-picker>
    <button class="theme-trigger" type="button" aria-haspopup="menu" :aria-expanded="open" @click="open = !open">
      <span class="theme-trigger__dot" aria-hidden="true" />
      <span>主题：{{ currentThemeLabel }}</span>
    </button>
    <div v-show="open" class="theme-menu">
      <div class="theme-options" role="radiogroup" aria-label="选择主题">
      <UiButton
        v-for="item in THEMES"
        :key="item.name"
        class="theme-option"
        :class="{ 'theme-option--current': item.name === theme.preview }"
        :data-theme-option="item.name"
        role="radio"
        :tabindex="item.name === theme.preview ? 0 : -1"
        :aria-checked="item.name === theme.preview"
        @click="selectTheme(item.name)"
        @keydown="handleOptionKeydown($event, THEMES.indexOf(item))"
      >
        <span class="theme-swatch" :data-theme-swatch="item.name" aria-hidden="true" />
        <span>{{ item.label }}</span>
        <UiIcon v-if="item.name === theme.preview" class="theme-check" name="check" />
      </UiButton>
      </div>
      <div v-if="theme.isDirty" class="theme-actions" data-theme-actions>
      <UiButton variant="primary" :loading="saving" data-theme-save @click="save">
        {{ saving ? '保存中…' : '设为站点默认' }}
      </UiButton>
      <UiButton variant="quiet" :disabled="saving" data-theme-cancel @click="theme.cancelPreview">
        取消预览
      </UiButton>
      </div>
      <p v-if="error" class="theme-error" role="alert">{{ error }}</p>
    </div>
  </div>
</template>

<style scoped>
.theme-picker {
  position: relative;
  display: grid;
  gap: var(--space-3);
}

.theme-trigger {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 2.5rem;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--c-line-strong);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 88%, var(--c-accent));
  color: var(--c-ink-muted);
  font: inherit;
  cursor: pointer;
}
.theme-trigger:hover, .theme-trigger:focus-visible { border-color: var(--c-accent); color: var(--c-accent); outline: none; }
.theme-trigger__dot { width: .5rem; height: .5rem; border-radius: 50%; background: var(--c-accent); }

.theme-label {
  margin: 0;
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.theme-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-2);
}

.theme-menu {
  position: absolute;
  z-index: 5;
  top: calc(100% + var(--space-2));
  right: 0;
  min-width: 13rem;
  padding: .5rem;
  border: 1px solid var(--c-line);
  border-radius: .875rem;
  background: var(--c-surface);
  box-shadow: var(--shadow-float);
}

.theme-option { justify-content: flex-start; min-width: 0; }
.theme-option:hover:not(:disabled) { border-color: var(--c-line-strong); background: var(--c-surface-sunken); color: var(--c-ink); }
.theme-option--current { border-color: var(--c-accent); background: var(--c-surface-sunken); color: var(--c-ink); }

.theme-check { width: 0.875rem; height: 0.875rem; margin-left: auto; }

.theme-swatch {
  flex: 0 0 auto;
  width: 0.75rem;
  height: 0.75rem;
  border: 1px solid var(--c-line-strong);
  border-radius: 50%;
  background: var(--c-accent);
}

.theme-swatch[data-theme-swatch='lamp'], .theme-swatch[data-theme-swatch='night-ink'] { background: var(--c-ink); }

.theme-actions {
  display: grid;
  margin-top: var(--space-3);
  padding-top: var(--space-3);
  border-top: 1px solid var(--c-line);
  gap: var(--space-2);
}

.theme-error {
  margin: 0;
  color: var(--c-danger);
  font-size: 0.8125rem;
}

@media (max-width: 20rem) { .theme-options { grid-template-columns: 1fr; } }
</style>
