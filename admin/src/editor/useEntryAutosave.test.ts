import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import { ApiClientError, NETWORK_ERROR } from '../api/errors'
import type { SaveCoordinator, SaveSnapshot } from './save-coordinator'
import { useEntryAutosave } from './useEntryAutosave'

describe('useEntryAutosave', () => {
  it('subscribes on mount and exposes reactive save state', async () => {
    const coordinator = createCoordinator()
    let autosave!: ReturnType<typeof useEntryAutosave>
    const wrapper = mount(
      defineComponent({
        setup() {
          autosave = useEntryAutosave(coordinator)
          return () => null
        },
      }),
    )
    const error = new ApiClientError({
      code: NETWORK_ERROR,
      status: 0,
      message: 'offline',
    })

    coordinator.emit({ status: 'offline', revision: 4, pendingFields: {}, error })
    await nextTick()

    expect(coordinator.subscribe).toHaveBeenCalledOnce()
    expect(autosave.status.value).toBe('offline')
    expect(autosave.revision.value).toBe(4)
    expect(autosave.error.value).toBe(error)
    await expect(autosave.flush()).resolves.toBeNull()
    expect(coordinator.flush).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('unsubscribes and disposes the coordinator on unmount', () => {
    const coordinator = createCoordinator()
    const wrapper = mount(
      defineComponent({
        setup() {
          useEntryAutosave(coordinator)
          return () => null
        },
      }),
    )

    wrapper.unmount()

    expect(coordinator.unsubscribe).toHaveBeenCalledOnce()
    expect(coordinator.dispose).toHaveBeenCalledOnce()
  })
})

function createCoordinator(): SaveCoordinator & {
  emit(snapshot: SaveSnapshot): void
  subscribe: ReturnType<typeof vi.fn>
  unsubscribe: ReturnType<typeof vi.fn>
  flush: ReturnType<typeof vi.fn>
  dispose: ReturnType<typeof vi.fn>
} {
  let listener: ((snapshot: SaveSnapshot) => void) | null = null
  const unsubscribe = vi.fn()
  const subscribe = vi.fn((nextListener: (snapshot: SaveSnapshot) => void) => {
    listener = nextListener
    return unsubscribe
  })

  return {
    update: vi.fn(),
    retry: vi.fn().mockResolvedValue(null),
    flush: vi.fn().mockResolvedValue(null),
    dispose: vi.fn().mockResolvedValue(undefined),
    subscribe,
    unsubscribe,
    emit(snapshot) {
      listener?.(snapshot)
    },
  }
}
