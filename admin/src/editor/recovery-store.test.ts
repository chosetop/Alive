import { afterEach, describe, expect, it, vi } from 'vitest'

import { EntryRecoveryStore, type RecoveryRecord } from './recovery-store'

const record: RecoveryRecord = {
  entryId: 42,
  revision: 3,
  fields: { title: '山中', content_md: '正文' },
  savedLocallyAt: '2026-08-26T12:00:00.000Z',
  syncState: 'pending',
}

describe('EntryRecoveryStore', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns a record saved for an entry', async () => {
    const store = new EntryRecoveryStore()

    await store.put(record)

    expect(await store.get(42)).toEqual(record)
  })

  it('replaces an older record when a newer local revision is saved', async () => {
    const store = new EntryRecoveryStore()
    const olderRecord: RecoveryRecord = {
      ...record,
      revision: 2,
      fields: { title: '旧标题', content_md: '旧正文' },
      savedLocallyAt: '2026-08-26T11:59:00.000Z',
    }

    await store.put(olderRecord)
    await store.put(record)

    expect(await store.get(42)).toEqual(record)
  })

  it('keeps a newer record when an older local revision reaches IndexedDB later', async () => {
    const store = new EntryRecoveryStore()
    const olderRecord: RecoveryRecord = {
      ...record,
      revision: 2,
      fields: { title: '旧标题', content_md: '旧正文' },
      savedLocallyAt: '2026-08-26T11:59:00.000Z',
    }

    await store.put(record)
    await store.put(olderRecord)

    expect(await store.get(42)).toEqual(record)
  })

  it('returns an empty field patch unchanged', async () => {
    const store = new EntryRecoveryStore()
    const emptyPatchRecord: RecoveryRecord = {
      ...record,
      entryId: 43,
      fields: {},
    }

    await store.put(emptyPatchRecord)

    expect(await store.get(43)).toEqual(emptyPatchRecord)
  })

  it('rejects a recovery record that IndexedDB cannot clone', async () => {
    const store = new EntryRecoveryStore()
    const uncloneableRecord: RecoveryRecord = {
      ...record,
      fields: { meta: { callback: () => undefined } },
    }

    await expect(store.put(uncloneableRecord)).rejects.toBeInstanceOf(DOMException)
  })

  it('lists every unsynced recovery record', async () => {
    const store = new EntryRecoveryStore()
    const conflictRecord: RecoveryRecord = {
      entryId: 7,
      revision: 5,
      fields: { title: '冲突草稿', content_md: '待处理正文' },
      savedLocallyAt: '2026-08-26T12:01:00.000Z',
      syncState: 'conflict',
    }

    await store.put(conflictRecord)
    await store.put(record)

    expect(await store.list()).toEqual([conflictRecord, record])
  })

  it('removes a recovery record after successful synchronization', async () => {
    const store = new EntryRecoveryStore()

    await store.put(record)
    await store.remove(42)

    expect(await store.get(42)).toBeNull()
  })

  it('rejects when an IndexedDB request errors', async () => {
    const requestError = new DOMException('Read failed', 'UnknownError')
    const request = {
      error: requestError,
      onerror: null,
      onsuccess: null,
      result: undefined,
    } as unknown as IDBRequest<RecoveryRecord | undefined>
    const transaction = {
      error: null,
      onabort: null,
      onerror: null,
      objectStore: () => ({ get: () => request }),
    } as unknown as IDBTransaction

    const { openRequest } = stubDatabase(transaction)
    const operation = new EntryRecoveryStore().get(42)
    openRequest.onsuccess?.call(openRequest, new Event('success'))
    queueMicrotask(() => request.onerror?.call(request, new Event('error')))

    await expect(operation).rejects.toBe(requestError)
  })

  it.each(['error', 'abort'] as const)('rejects when an IndexedDB transaction %ss', async (eventName) => {
    const transactionError = new DOMException(`Transaction ${eventName}`, 'AbortError')
    const request = {
      error: null,
      onerror: null,
      onsuccess: null,
      result: undefined,
    } as unknown as IDBRequest<RecoveryRecord | undefined>
    const transaction = {
      error: transactionError,
      onabort: null,
      onerror: null,
      objectStore: () => ({ get: () => request }),
    } as unknown as IDBTransaction

    const { openRequest } = stubDatabase(transaction)
    const operation = new EntryRecoveryStore().get(42)
    openRequest.onsuccess?.call(openRequest, new Event('success'))
    queueMicrotask(() => {
      if (eventName === 'error') {
        transaction.onerror?.call(transaction, new Event('error'))
      } else {
        transaction.onabort?.call(transaction, new Event('abort'))
      }
    })

    await expect(operation).rejects.toBe(transactionError)
  })

  it('rejects a blocked upgrade and closes a connection that opens later', async () => {
    const close = vi.fn()
    const database = {
      close,
      transaction: vi.fn(),
    } as unknown as IDBDatabase
    const { openRequest } = stubDatabase({} as IDBTransaction, database)
    const operation = new EntryRecoveryStore().get(42)

    openRequest.onblocked?.call(openRequest, new Event('blocked') as IDBVersionChangeEvent)
    const outcome = await Promise.race([
      operation.then(
        () => 'resolved',
        () => 'rejected',
      ),
      new Promise<'pending'>((resolve) => setTimeout(() => resolve('pending'), 0)),
    ])

    expect(outcome).toBe('rejected')
    openRequest.onsuccess?.call(openRequest, new Event('success'))
    expect(close).toHaveBeenCalledTimes(1)
  })
})

function stubDatabase(
  transaction: IDBTransaction,
  database: IDBDatabase = {
    close: vi.fn(),
    transaction: vi.fn(() => transaction),
  } as unknown as IDBDatabase,
): { openRequest: IDBOpenDBRequest } {
  const openRequest = {
    error: null,
    onerror: null,
    onupgradeneeded: null,
    onsuccess: null,
    result: database,
  } as unknown as IDBOpenDBRequest

  vi.stubGlobal('indexedDB', { open: vi.fn(() => openRequest) })
  return { openRequest }
}
