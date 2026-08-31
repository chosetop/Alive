import type { SayingView } from '~/types'

const KEY = 'alive:sayings-view:v1'
const VALUES: SayingView[] = ['stream', 'wall', 'focus']

export function resolveSayingView(value: unknown, fallback: SayingView = 'stream'): SayingView {
  return typeof value === 'string' && VALUES.includes(value as SayingView) ? (value as SayingView) : fallback
}

export function useSayingView(defaultView: SayingView = 'stream') {
  const view = useState<SayingView>('sayings-view', () => defaultView)
  if (import.meta.client) {
    const saved = window.localStorage.getItem(KEY)
    if (saved) view.value = resolveSayingView(saved, defaultView)
  }
  function choose(next: SayingView): void {
    view.value = resolveSayingView(next, defaultView)
    if (import.meta.client) window.localStorage.setItem(KEY, view.value)
  }
  return { view, choose }
}
