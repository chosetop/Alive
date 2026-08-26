import type { EntryPatchFields } from '../types/api'

const databaseName = 'alive-admin'
const databaseVersion = 1
const storeName = 'entry-recovery'

export interface RecoveryRecord {
  entryId: number
  revision: number
  fields: EntryPatchFields
  savedLocallyAt: string
  syncState: 'pending' | 'conflict'
}

export class EntryRecoveryStore {
  async get(entryId: number): Promise<RecoveryRecord | null> {
    return this.withDatabase(async (database) => {
      const record = await runRequest(database, 'readonly', (store) => store.get(entryId))
      return record ?? null
    })
  }

  async put(record: RecoveryRecord): Promise<void> {
    await this.withDatabase((database) => putIfCurrent(database, record))
  }

  async remove(entryId: number): Promise<void> {
    await this.withDatabase(async (database) => {
      await runRequest(database, 'readwrite', (store) => store.delete(entryId))
    })
  }

  async list(): Promise<RecoveryRecord[]> {
    return this.withDatabase((database) => runRequest(database, 'readonly', (store) => store.getAll()))
  }

  private async withDatabase<T>(operation: (database: IDBDatabase) => Promise<T>): Promise<T> {
    const database = await openDatabase()

    try {
      return await operation(database)
    } finally {
      database.close()
    }
  }
}

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(databaseName, databaseVersion)
    let settled = false

    request.onupgradeneeded = () => {
      const database = request.result
      if (!database.objectStoreNames.contains(storeName)) {
        database.createObjectStore(storeName, { keyPath: 'entryId' })
      }
    }
    request.onerror = () => {
      if (!settled) {
        settled = true
        reject(request.error ?? new Error('Opening IndexedDB failed'))
      }
    }
    request.onblocked = () => {
      if (!settled) {
        settled = true
        reject(new Error('Opening IndexedDB was blocked'))
      }
    }
    request.onsuccess = () => {
      if (settled) {
        request.result.close()
      } else {
        settled = true
        resolve(request.result)
      }
    }
  })
}

function putIfCurrent(database: IDBDatabase, record: RecoveryRecord): Promise<void> {
  return new Promise((resolve, reject) => {
    let settled = false
    const settle = (callback: () => void) => {
      if (!settled) {
        settled = true
        callback()
      }
    }

    let transaction: IDBTransaction
    try {
      transaction = database.transaction(storeName, 'readwrite')
      const store = transaction.objectStore(storeName)
      const getRequest = store.get(record.entryId)

      getRequest.onerror = () => {
        settle(() => reject(getRequest.error ?? new Error('IndexedDB request failed')))
      }
      getRequest.onsuccess = () => {
        const current = getRequest.result
        if (current === undefined || record.revision >= current.revision) {
          const putRequest = store.put(record)
          putRequest.onerror = () => {
            settle(() => reject(putRequest.error ?? new Error('IndexedDB request failed')))
          }
        }
      }
      transaction.onerror = () => {
        settle(() => reject(transaction.error ?? new Error('IndexedDB transaction failed')))
      }
      transaction.onabort = () => {
        settle(() => reject(transaction.error ?? new Error('IndexedDB transaction aborted')))
      }
      transaction.oncomplete = () => {
        settle(resolve)
      }
    } catch (error) {
      settle(() => reject(error))
    }
  })
}

function runRequest<T>(
  database: IDBDatabase,
  mode: IDBTransactionMode,
  createRequest: (store: IDBObjectStore) => IDBRequest<T>,
): Promise<T> {
  return new Promise((resolve, reject) => {
    let result: T
    let settled = false
    const settle = (callback: () => void) => {
      if (!settled) {
        settled = true
        callback()
      }
    }

    let transaction: IDBTransaction
    try {
      transaction = database.transaction(storeName, mode)
      const request = createRequest(transaction.objectStore(storeName))

      request.onerror = () => {
        settle(() => reject(request.error ?? new Error('IndexedDB request failed')))
      }
      request.onsuccess = () => {
        result = request.result
      }
      transaction.onerror = () => {
        settle(() => reject(transaction.error ?? new Error('IndexedDB transaction failed')))
      }
      transaction.onabort = () => {
        settle(() => reject(transaction.error ?? new Error('IndexedDB transaction aborted')))
      }
      transaction.oncomplete = () => {
        settle(() => resolve(result))
      }
    } catch (error) {
      settle(() => reject(error))
    }
  })
}
