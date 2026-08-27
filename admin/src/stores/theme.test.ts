import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { updateSiteSettings } from '../api/site'
import { useThemeStore } from './theme'

vi.mock('../api/site', () => ({
  updateSiteSettings: vi.fn(),
}))

describe('theme store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.documentElement.removeAttribute('data-theme')
    vi.mocked(updateSiteSettings).mockReset()
  })

  it('previews locally without changing the saved site default', () => {
    const store = useThemeStore()
    store.hydrate({ default_theme: 'ink', revision: 1, updated_at: '' })

    store.previewTheme('codex-lavender')

    expect(store.siteDefault).toBe('ink')
    expect(store.preview).toBe('codex-lavender')
    expect(store.isDirty).toBe(true)
    expect(document.documentElement.dataset.theme).toBe('codex-lavender')
    expect(updateSiteSettings).not.toHaveBeenCalled()
  })

  it('changes the saved default only after the update succeeds', async () => {
    vi.mocked(updateSiteSettings).mockResolvedValue({
      default_theme: 'codex-lavender',
      revision: 2,
      updated_at: '',
    })
    const store = useThemeStore()
    store.hydrate({ default_theme: 'ink', revision: 1, updated_at: '' })
    store.previewTheme('codex-lavender')

    await store.saveDefault()

    expect(updateSiteSettings).toHaveBeenCalledWith({ default_theme: 'codex-lavender', revision: 1 })
    expect(store.siteDefault).toBe('codex-lavender')
    expect(store.revision).toBe(2)
    expect(store.isDirty).toBe(false)
  })

  it('keeps the preview when saving fails', async () => {
    vi.mocked(updateSiteSettings).mockRejectedValue(new Error('offline'))
    const store = useThemeStore()
    store.hydrate({ default_theme: 'ink', revision: 1, updated_at: '' })
    store.previewTheme('lamp')

    await expect(store.saveDefault()).rejects.toThrow('offline')

    expect(store.siteDefault).toBe('ink')
    expect(store.preview).toBe('lamp')
    expect(store.isDirty).toBe(true)
    expect(document.documentElement.dataset.theme).toBe('lamp')
  })

  it('cancels preview and restores the saved default', () => {
    const store = useThemeStore()
    store.hydrate({ default_theme: 'lamp', revision: 3, updated_at: '' })
    store.previewTheme('codex-lavender')

    store.cancelPreview()

    expect(store.preview).toBe('lamp')
    expect(store.isDirty).toBe(false)
    expect(document.documentElement.dataset.theme).toBe('lamp')
  })
})
