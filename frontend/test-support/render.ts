import { createSSRApp, defineComponent, h, type Component } from 'vue'
import { renderToString } from 'vue/server-renderer'

function resolveHref(to: unknown): string {
  if (typeof to === 'string') return to
  if (to && typeof to === 'object' && 'path' in to && typeof (to as { path?: unknown }).path === 'string') {
    return (to as { path: string }).path
  }
  return '#'
}

export async function renderComponent(component: Component, props: Record<string, unknown> = {}): Promise<string> {
  const app = createSSRApp({
    render: () => h(component, props as never),
  })

  app.component(
    'NuxtLink',
    defineComponent({
      name: 'NuxtLinkStub',
      props: {
        to: {
          type: [String, Object],
          default: '#',
        },
      },
      setup(linkProps, { slots }) {
        return () => h('a', { href: resolveHref(linkProps.to) }, slots.default?.())
      },
    }),
  )

  return renderToString(app)
}
