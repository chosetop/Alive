import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it } from 'vitest'

import { useThemeStore } from '../../stores/theme'
import ThemePicker from './ThemePicker.vue'

describe('ThemePicker', () => {
  it('shows the preserved 薰衣草 label in the theme picker', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const theme = useThemeStore()
    theme.hydrate({ default_theme: 'ink', revision: 1, updated_at: '' })

    const wrapper = mount(ThemePicker, { global: { plugins: [pinia] } })

    // This fails if the shared manifest stops driving the visible control and
    // 薰衣草 becomes hidden behind a local, incomplete theme list.
    expect(wrapper.text()).toContain('薰衣草')
  })
})
