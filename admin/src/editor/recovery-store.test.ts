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
})

function stubDatabase(transaction: IDBTransaction): { openRequest: IDBOpenDBRequest } {
  const database = {
    close: vi.fn(),
    transaction: vi.fn(() => transaction),
  } as unknown as IDBDatabase
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
