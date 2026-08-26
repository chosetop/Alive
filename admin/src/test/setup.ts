import 'fake-indexeddb/auto'

import { afterEach } from 'vitest'

const databaseName = 'alive-admin'

afterEach(async () => {
  await new Promise<void>((resolve, reject) => {
    const request = indexedDB.deleteDatabase(databaseName)
    request.onsuccess = () => resolve()
    request.onerror = () => reject(request.error)
    request.onblocked = () => reject(new Error(`Deleting ${databaseName} was blocked`))
  })
})
