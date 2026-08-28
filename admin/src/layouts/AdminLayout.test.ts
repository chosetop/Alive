import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it } from 'vitest'

import AdminLayout from './AdminLayout.vue'

const page = { template: '<div />' }
const wrappers: VueWrapper[] = []

async function mountAdminLayout(path: string): Promise<VueWrapper> {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard', name: 'dashboard', component: page },
      { path: '/entries', name: 'entries', component: page },
      { path: '/categories', name: 'categories', component: page },
      { path: '/entries/new', name: 'entry-new', component: page },
    ],
  })
  await router.push(path)

  const wrapper = mount(AdminLayout, {
    global: {
      plugins: [createPinia(), router],
      stubs: { RouterView: { template: '<div data-page-content />' } },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
})

describe('AdminLayout', () => {
  it('renders the active navigation state with a non-color cue', async () => {
    const wrapper = await mountAdminLayout('/entries')
    const active = wrapper.get('a[aria-current="page"]')

    // Removing the state marker or its cursor leaves the selected destination
    // distinguishable only by colour, which is not enough for the nav contract.
    expect(active.attributes('data-active')).toBe('true')
    expect(active.find('[data-alive-cursor]').exists()).toBe(true)
  })

  it('uses the current route label in the context bar', async () => {
    const wrapper = await mountAdminLayout('/categories')

    // A static wordmark would not tell a person which utility section is open.
    expect(wrapper.get('[data-page-context]').text()).toBe('Categories')
  })

  it('uses a mobile navigation trigger with an accessible label', async () => {
    const wrapper = await mountAdminLayout('/entries')

    // An icon without this name is announced only as an anonymous button.
    expect(wrapper.get('[data-mobile-nav]').attributes('aria-label')).toBe('打开主导航')
  })
})
