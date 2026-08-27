import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { updateSiteSettings, type SiteSettings } from '../api/site'

export const useThemeStore = defineStore('theme', () => {
  const siteDefault = ref('ink')
  const preview = ref('ink')
  const revision = ref(1)
  const updatedAt = ref('')
  const isDirty = computed(() => siteDefault.value !== preview.value)

  function applyTheme(theme: string): void {
    document.documentElement.dataset.theme = theme
  }

  function hydrate(settings: SiteSettings): void {
    siteDefault.value = settings.default_theme
    preview.value = settings.default_theme
    revision.value = settings.revision
    updatedAt.value = settings.updated_at
    applyTheme(preview.value)
  }

  function previewTheme(theme: string): void {
    preview.value = theme
    applyTheme(theme)
  }

  async function saveDefault(): Promise<void> {
    const saved = await updateSiteSettings({
      default_theme: preview.value,
      revision: revision.value,
    })
    siteDefault.value = saved.default_theme
    preview.value = saved.default_theme
    revision.value = saved.revision
    updatedAt.value = saved.updated_at
    applyTheme(preview.value)
  }

  function cancelPreview(): void {
    preview.value = siteDefault.value
    applyTheme(preview.value)
  }

  return {
    siteDefault,
    preview,
    revision,
    updatedAt,
    isDirty,
    hydrate,
    previewTheme,
    saveDefault,
    cancelPreview,
  }
})
