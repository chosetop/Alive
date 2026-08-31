import { describe, expect, it } from 'vitest'

import {
  consumeSayingAnchor,
  loadSayingViewPreference,
  rememberSayingAnchor,
  resolveSayingView,
  saveSayingViewPreference,
} from './useSayingView'

describe('resolveSayingView', () => {
  it('keeps a valid explicit choice', () => {
    expect(resolveSayingView('wall', 'stream')).toBe('wall')
  })

  it('falls back to the site default when the explicit choice is invalid', () => {
    expect(resolveSayingView('bogus', 'focus')).toBe('focus')
  })

  it('falls back to stream when both inputs are invalid', () => {
    expect(resolveSayingView('bogus', 'also-bogus')).toBe('stream')
  })
})

describe('sayings view preference storage', () => {
  function createStorage(initial: Record<string, string> = {}) {
    const entries = new Map(Object.entries(initial))
    return {
      getItem: (key: string) => entries.get(key) ?? null,
      setItem: (key: string, value: string) => {
        entries.set(key, value)
      },
    }
  }

  it('reads a valid stored preference', () => {
    const storage = createStorage({ 'alive:sayings-view:v1': JSON.stringify({ version: 1, view: 'focus' }) })

    expect(loadSayingViewPreference(storage, 'stream')).toBe('focus')
  })

  it('ignores a legacy stored preference version', () => {
    const storage = createStorage({ 'alive:sayings-view:v1': JSON.stringify({ version: 0, view: 'wall' }) })

    expect(loadSayingViewPreference(storage, 'stream')).toBe('stream')
  })

  it('ignores malformed stored preference data', () => {
    const storage = createStorage({ 'alive:sayings-view:v1': 'not json' })

    expect(loadSayingViewPreference(storage, 'wall')).toBe('wall')
  })

  it('persists an explicit choice without writing the fallback', () => {
    const storage = createStorage()

    saveSayingViewPreference(storage, 'focus')

    expect(storage.getItem('alive:sayings-view:v1')).toBe(JSON.stringify({ version: 1, view: 'focus' }))
  })
})

describe('saying return anchors', () => {
  function createHistory() {
    let currentState: unknown = {}
    return {
      get state() {
        return currentState
      },
      replaceState(nextState: unknown, _title: string) {
        currentState = nextState
      },
    }
  }

  it('remembers the current saying and its offset', () => {
    const history = createHistory()

    rememberSayingAnchor(history, 'abc123', 240)

    expect(history.state).toEqual({ aliveSayingAnchor: { shortId: 'abc123', offsetTop: 240 } })
  })

  it('consumes a stored anchor once', () => {
    const history = createHistory()
    history.replaceState({ aliveSayingAnchor: { shortId: 'abc123', offsetTop: 240 } }, '')

    expect(consumeSayingAnchor(history)).toEqual({ shortId: 'abc123', offsetTop: 240 })
    expect(history.state).toEqual({})
    expect(consumeSayingAnchor(history)).toBeNull()
  })
})
