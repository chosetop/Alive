import { onMounted, onUnmounted, ref, shallowRef, type Ref, type ShallowRef } from 'vue'

import type { ApiClientError } from '../api/errors'
import type { EntryDetail } from '../types/api'
import type { SaveCoordinator, SaveStatus } from './save-coordinator'

export interface EntryAutosave {
  status: Ref<SaveStatus>
  revision: Ref<number>
  error: ShallowRef<ApiClientError | null>
  flush(): Promise<EntryDetail | null>
}

export function useEntryAutosave(coordinator: SaveCoordinator): EntryAutosave {
  const status = ref<SaveStatus>('saved')
  const revision = ref(0)
  const error = shallowRef<ApiClientError | null>(null)
  let unsubscribe: (() => void) | null = null

  onMounted(() => {
    unsubscribe = coordinator.subscribe((snapshot) => {
      status.value = snapshot.status
      revision.value = snapshot.revision
      error.value = snapshot.error
    })
  })

  onUnmounted(() => {
    unsubscribe?.()
    unsubscribe = null
    void coordinator.dispose()
  })

  return {
    status,
    revision,
    error,
    flush: () => coordinator.flush(),
  }
}
