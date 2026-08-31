import { computed, unref, type ComputedRef, type Ref } from 'vue'

import type { SayingView } from '~/types'

const STORAGE_KEY = 'alive:sayings-view:v1'

type SayingViewRecord = {
  version: 1
  view: SayingView
}

type StorageLike = Pick<Storage, 'getItem' | 'setItem'>
type HistoryLike = {
  state: unknown
  replaceState: (state: unknown, title?: string, url?: string | null) => void
}

const VALUES: readonly SayingView[] = ['stream', 'wall', 'focus']

function isView(value: unknown): value is SayingView {
  return typeof value === 'string' && VALUES.includes(value as SayingView)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

export function resolveSayingView(value: unknown, fallback: SayingView = 'stream'): SayingView {
  if (isView(value)) return value
  return isView(fallback) ? fallback : 'stream'
}

export function loadSayingViewPreference(storage: StorageLike, fallback: SayingView): SayingView {
  try {
    const raw = storage.getItem(STORAGE_KEY)
    if (raw === null || raw === '') return fallback

    const parsed = JSON.parse(raw) as Partial<SayingViewRecord>
    if (parsed.version !== 1) return fallback

    return resolveSayingView(parsed.view, fallback)
  } catch {
    return fallback
  }
}

export function saveSayingViewPreference(storage: StorageLike, view: SayingView): void {
  try {
    storage.setItem(STORAGE_KEY, JSON.stringify({ version: 1, view } satisfies SayingViewRecord))
  } catch {
    // Storage can be unavailable in private mode or during tests; the choice is
    // still valid even when it cannot be persisted.
  }
}

export function rememberSayingAnchor(history: HistoryLike, shortId: string, offsetTop: number): void {
  history.replaceState(
    {
      ...(isRecord(history.state) ? history.state : {}),
      aliveSayingAnchor: { shortId, offsetTop },
    },
    '',
  )
}

export function consumeSayingAnchor(history: HistoryLike): { shortId: string; offsetTop: number } | null {
  if (!isRecord(history.state)) return null

  const anchor = history.state.aliveSayingAnchor
  const nextState = { ...history.state } as Record<string, unknown>
  delete nextState.aliveSayingAnchor
  history.replaceState(nextState, '')

  if (!isRecord(anchor)) return null
  const shortId = anchor.shortId
  const offsetTop = anchor.offsetTop
  if (typeof shortId !== 'string' || typeof offsetTop !== 'number') return null

  return { shortId, offsetTop }
}

export function useSayingView(siteDefault: Ref<string> | ComputedRef<string> | string = 'stream') {
  const resolvedDefault = computed<SayingView>(() => resolveSayingView(unref(siteDefault), 'stream'))
  const view = useState<SayingView>('sayings-view', () => resolvedDefault.value)

  if (import.meta.client && typeof window !== 'undefined') {
    view.value = loadSayingViewPreference(window.localStorage, resolvedDefault.value)
  }

  function choose(next: SayingView): void {
    const selected = resolveSayingView(next, resolvedDefault.value)
    view.value = selected
    if (import.meta.client && typeof window !== 'undefined') {
      saveSayingViewPreference(window.localStorage, selected)
    }
  }

  return { view, choose, resolvedDefault }
}
