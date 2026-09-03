import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import { useThemeStore } from '../../stores/theme'
import ThemePicker from './ThemePicker.vue'

const site = vi.hoisted(() => ({ update: vi.fn() }))

vi.mock('../../api/site', () => ({
  getSiteSettings: vi.fn(),
  updateSiteSettings: site.update,
}))

describe('ThemePicker', () => {
  it('shows the preserved 薰衣草 label in the theme picker', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const theme = useThemeStore()
    theme.hydrate({ default_theme: 'ink', revision: 1, updated_at: '' })

    const wrapper = mount(ThemePicker, { attachTo: document.body, global: { plugins: [pinia] } })

    // This fails if the shared manifest stops driving the visible control and
    // 薰衣草 becomes hidden behind a local, incomplete theme list.
    expect(wrapper.text()).toContain('薰衣草')
  })

  it('roves keyboard selection through the manifest themes', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const theme = useThemeStore()
    theme.hydrate({ default_theme: 'ink', revision: 1, updated_at: '' })

    const wrapper = mount(ThemePicker, { attachTo: document.body, global: { plugins: [pinia] } })
    const radios = wrapper.findAll('[role="radio"]')

    // Removing the roving tab stop makes every visual option reachable by Tab,
    // while the advertised radio group still provides no keyboard selection.
    expect(radios.map((radio) => radio.attributes('tabindex'))).toEqual(['0', '-1', '-1', '-1'])

    const moves = [
      { key: 'ArrowRight', from: 0, to: 1, theme: 'lamp' },
      { key: 'ArrowDown', from: 1, to: 2, theme: 'codex-lavender' },
      { key: 'ArrowLeft', from: 2, to: 1, theme: 'lamp' },
      { key: 'ArrowUp', from: 1, to: 0, theme: 'ink' },
      { key: 'End', from: 0, to: 3, theme: 'night-ink' },
      { key: 'Home', from: 3, to: 0, theme: 'ink' },
    ]

    for (const move of moves) {
      await radios[move.from].trigger('keydown', { key: move.key })
      await nextTick()

      expect(theme.preview).toBe(move.theme)
      expect(radios[move.to].attributes('aria-checked')).toBe('true')
      expect(radios[move.to].attributes('tabindex')).toBe('0')
      expect(document.activeElement).toBe(radios[move.to].element)
    }

    expect(wrapper.find('[data-theme-actions]').exists()).toBe(false)
  })

  it('closes the menu after the selected theme is saved', async () => {
    site.update.mockResolvedValue({
      default_theme: 'lamp',
      revision: 2,
      updated_at: '2026-09-03T00:00:00Z',
    })
    const pinia = createPinia()
    setActivePinia(pinia)
    useThemeStore().hydrate({ default_theme: 'ink', revision: 1, updated_at: '' })
    const wrapper = mount(ThemePicker, { attachTo: document.body, global: { plugins: [pinia] } })

    await wrapper.get('.theme-trigger').trigger('click')
    await wrapper.get('[data-theme-option="lamp"]').trigger('click')
    await wrapper.get('[data-theme-save]').trigger('click')
    await flushPromises()
    await nextTick()

    expect(site.update).toHaveBeenCalledWith({ default_theme: 'lamp', revision: 1 })
    expect(wrapper.get('.theme-menu').isVisible()).toBe(false)
    wrapper.unmount()
  })
})
