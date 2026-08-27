<script setup lang="ts">
import { computed, ref } from 'vue'

import { THEMES, type ThemeName } from '@alive/theme'
import { toUserMessage } from '../../api'
import { useThemeStore } from '../../stores/theme'
import { UiButton, UiIcon } from '../ui'

const theme = useThemeStore()
const saving = ref(false)
const error = ref<string | null>(null)

const currentThemeLabel = computed(
  () => THEMES.find((item) => item.name === theme.preview)?.label ?? '',
)

function selectTheme(name: string): void {
  if (!THEMES.some((item) => item.name === name)) return
  theme.previewTheme(name as ThemeName)
  error.value = null
}

async function save(): Promise<void> {
  if (!theme.isDirty || saving.value) return
  saving.value = true
  error.value = null
  try {
    await theme.saveDefault()
  } catch (saveFailure) {
    error.value = toUserMessage(saveFailure)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="theme-picker" data-theme-picker>
    <p class="theme-label">主题：{{ currentThemeLabel }}</p>
    <div class="theme-options" role="radiogroup" aria-label="选择主题">
      <UiButton
        v-for="item in THEMES"
        :key="item.name"
        class="theme-option"
        :class="{ 'theme-option--current': item.name === theme.preview }"
        :data-theme-option="item.name"
        role="radio"
        :aria-checked="item.name === theme.preview"
        @click="selectTheme(item.name)"
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
</template>

<style scoped>
.theme-picker {
  display: grid;
  gap: var(--space-3);
}

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

.theme-option { justify-content: flex-start; min-width: 0; }
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
  gap: var(--space-2);
}

.theme-error {
  margin: 0;
  color: var(--c-danger);
  font-size: 0.8125rem;
}

@media (max-width: 20rem) { .theme-options { grid-template-columns: 1fr; } }
</style>
