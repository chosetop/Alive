import { ApiClientError, isApiClientError, NETWORK_ERROR } from '../api/errors'
import type { EntryDetail, EntryPatchFields, EntryUpdateRequest } from '../types/api'
import type { EntryRecoveryStore } from './recovery-store'

export type SaveStatus = 'saved' | 'pending' | 'saving' | 'offline' | 'error' | 'conflict'

export interface SaveSnapshot {
  status: SaveStatus
  revision: number
  pendingFields: EntryPatchFields | null
  error: ApiClientError | null
}

export interface SaveCoordinator {
  update(fields: EntryPatchFields): void
  flush(): Promise<EntryDetail | null>
  retry(): Promise<EntryDetail | null>
  subscribe(listener: (snapshot: SaveSnapshot) => void): () => void
  dispose(): Promise<void>
}

interface SaveCoordinatorOptions {
  entryId: number
  initialRevision: number
  waitMs: number
  recoveryStore: Pick<EntryRecoveryStore, 'put' | 'remove'>
  save(entryId: number, body: EntryUpdateRequest): Promise<EntryDetail>
}

export function createSaveCoordinator(options: SaveCoordinatorOptions): SaveCoordinator {
  const listeners = new Set<(snapshot: SaveSnapshot) => void>()
  let snapshot: SaveSnapshot = {
    status: 'saved',
    revision: options.initialRevision,
    pendingFields: null,
    error: null,
  }
  let timer: ReturnType<typeof setTimeout> | null = null
  let inFlight: Promise<EntryDetail | null> | null = null
  let disposed = false

  const publish = (next: SaveSnapshot): void => {
    snapshot = next
    for (const listener of listeners) listener(copySnapshot(snapshot))
  }

  const clearTimer = (): void => {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  const schedule = (): void => {
    clearTimer()
    timer = setTimeout(() => {
      timer = null
      void startSave()
    }, options.waitMs)
  }

  const restoreFailedFields = (fields: EntryPatchFields): EntryPatchFields => ({
    ...fields,
    ...(snapshot.pendingFields ?? {}),
  })

  const fail = async (
    caught: unknown,
    fields: EntryPatchFields,
    syncState: 'pending' | 'conflict',
  ): Promise<null> => {
    const error = toApiClientError(caught)
    let pendingFields = restoreFailedFields(fields)
    publish({ ...snapshot, status: 'saving', pendingFields: null, error: null })

    while (true) {
      try {
        await options.recoveryStore.put({
          entryId: options.entryId,
          revision: snapshot.revision,
          fields: pendingFields,
          savedLocallyAt: new Date().toISOString(),
          syncState,
        })
      } catch (recoveryError) {
        publish({
          status: 'error',
          revision: snapshot.revision,
          pendingFields: restoreFailedFields(pendingFields),
          error: toApiClientError(recoveryError),
        })
        return null
      }

      if (snapshot.pendingFields === null) break
      pendingFields = { ...pendingFields, ...snapshot.pendingFields }
      publish({ ...snapshot, status: 'saving', pendingFields: null, error: null })
    }

    publish({
      status:
        syncState === 'conflict'
          ? 'conflict'
          : error.code === NETWORK_ERROR
            ? 'offline'
            : 'error',
      revision: snapshot.revision,
      pendingFields,
      error,
    })
    return null
  }

  const savePending = async (): Promise<EntryDetail | null> => {
    let latestEntry: EntryDetail | null = null
    let recoverableFields: EntryPatchFields = {}

    while (true) {
      while (snapshot.pendingFields !== null) {
        const fields = snapshot.pendingFields
        publish({ ...snapshot, status: 'saving', pendingFields: null, error: null })

        try {
          await options.recoveryStore.put({
            entryId: options.entryId,
            revision: snapshot.revision,
            fields,
            savedLocallyAt: new Date().toISOString(),
            syncState: 'pending',
          })
        } catch (error) {
          return fail(error, fields, 'pending')
        }

        let savedEntry: EntryDetail
        try {
          savedEntry = await options.save(options.entryId, {
            revision: snapshot.revision,
            ...fields,
          })
        } catch (error) {
          const syncState = isApiClientError(error) && error.status === 409 ? 'conflict' : 'pending'
          return fail(error, fields, syncState)
        }

        latestEntry = savedEntry
        recoverableFields = fields
        publish({ ...snapshot, status: 'saving', revision: savedEntry.revision, error: null })
      }

      try {
        await options.recoveryStore.remove(options.entryId)
      } catch (error) {
        return fail(error, recoverableFields, 'pending')
      }
      if (snapshot.pendingFields !== null) continue

      publish({ status: 'saved', revision: snapshot.revision, pendingFields: null, error: null })
      return latestEntry
    }
  }

  const startSave = (): Promise<EntryDetail | null> => {
    clearTimer()
    if (inFlight !== null) return inFlight
    if (snapshot.pendingFields === null) return Promise.resolve(null)

    inFlight = savePending().finally(() => {
      inFlight = null
      if (!disposed && snapshot.status === 'saving' && snapshot.pendingFields !== null) schedule()
    })
    return inFlight
  }

  return {
    update(fields) {
      if (disposed) return

      const pendingFields = { ...(snapshot.pendingFields ?? {}), ...fields }
      if (snapshot.status === 'conflict') {
        publish({ ...snapshot, pendingFields })
        return
      }

      publish({ ...snapshot, status: inFlight === null ? 'pending' : 'saving', pendingFields, error: null })
      if (inFlight === null) schedule()
    },

    flush() {
      if (disposed) return Promise.resolve(null)
      return startSave()
    },

    retry() {
      if (disposed) return Promise.resolve(null)
      return startSave()
    },

    subscribe(listener) {
      if (disposed) return () => undefined
      listeners.add(listener)
      listener(copySnapshot(snapshot))
      return () => listeners.delete(listener)
    },

    async dispose() {
      if (disposed) return
      disposed = true
      clearTimer()

      if (inFlight !== null) {
        await inFlight
      } else if (snapshot.status === 'pending') {
        await startSave()
      }
      listeners.clear()
    },
  }
}

function copySnapshot(snapshot: SaveSnapshot): SaveSnapshot {
  return {
    ...snapshot,
    pendingFields: snapshot.pendingFields === null ? null : { ...snapshot.pendingFields },
  }
}

function toApiClientError(error: unknown): ApiClientError {
  if (isApiClientError(error)) return error
  return new ApiClientError({
    code: 'INTERNAL',
    status: 0,
    message: error instanceof Error ? error.message : 'Autosave failed',
  })
}
