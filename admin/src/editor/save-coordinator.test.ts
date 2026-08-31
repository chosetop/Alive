import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiClientError, NETWORK_ERROR } from '../api/errors'
import type { EntryDetail, EntryUpdateRequest } from '../types/api'
import type { RecoveryRecord } from './recovery-store'
import { createSaveCoordinator, type SaveSnapshot } from './save-coordinator'

describe('createSaveCoordinator', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-26T12:00:00.000Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('debounces changes and writes recovery state before the network save', async () => {
    const events: string[] = []
    const recoveryStore = createRecoveryStore({
      put: vi.fn(async () => {
        events.push('local')
      }),
    })
    const save = vi.fn(async () => {
      events.push('network')
      return entryAtRevision(2)
    })
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })

    coordinator.update({ title: '新的标题' })
    await vi.advanceTimersByTimeAsync(999)
    expect(save).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)

    expect(events.slice(0, 2)).toEqual(['local', 'network'])
    expect(recoveryStore.put).toHaveBeenCalledWith({
      entryId: 42,
      revision: 1,
      fields: { title: '新的标题' },
      savedLocallyAt: '2026-08-26T12:00:01.000Z',
      syncState: 'pending',
    })
    expect(save).toHaveBeenCalledWith(42, { revision: 1, title: '新的标题' })
    expect(recoveryStore.remove).toHaveBeenCalledWith(42)
  })

  it('merges field patches received during the debounce window', async () => {
    const recoveryStore = createRecoveryStore()
    const save = vi.fn().mockResolvedValue(entryAtRevision(2))
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })

    coordinator.update({ title: '标题', summary: '旧摘要' })
    coordinator.update({ summary: '新摘要', content_md: '正文' })
    await vi.advanceTimersByTimeAsync(1000)

    expect(save).toHaveBeenCalledOnce()
    expect(save).toHaveBeenCalledWith(42, {
      revision: 1,
      title: '标题',
      summary: '新摘要',
      content_md: '正文',
    })
  })

  it('runs only one request at a time and saves changes queued during that request next', async () => {
    const firstSave = deferred<EntryDetail>()
    const recoveryStore = createRecoveryStore()
    const save = vi
      .fn<(entryId: number, body: EntryUpdateRequest) => Promise<EntryDetail>>()
      .mockImplementationOnce(() => firstSave.promise)
      .mockResolvedValueOnce(entryAtRevision(3))
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })

    coordinator.update({ title: '第一版' })
    await vi.advanceTimersByTimeAsync(1000)
    coordinator.update({ content_md: '保存期间写入' })
    await vi.advanceTimersByTimeAsync(1000)

    expect(save).toHaveBeenCalledOnce()
    firstSave.resolve(entryAtRevision(2))
    await vi.runAllTimersAsync()

    expect(save).toHaveBeenCalledTimes(2)
    expect(save).toHaveBeenNthCalledWith(1, 42, { revision: 1, title: '第一版' })
    expect(save).toHaveBeenNthCalledWith(2, 42, {
      revision: 2,
      content_md: '保存期间写入',
    })
    expect(recoveryStore.remove).toHaveBeenCalledOnce()
  })

  it('saves a change queued while successful recovery cleanup is still in flight', async () => {
    const cleanup = deferred<void>()
    const recoveryStore = createRecoveryStore({
      remove: vi.fn(() => cleanup.promise),
    })
    const save = vi
      .fn<(entryId: number, body: EntryUpdateRequest) => Promise<EntryDetail>>()
      .mockResolvedValueOnce(entryAtRevision(2))
      .mockResolvedValueOnce(entryAtRevision(3))
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })

    coordinator.update({ title: '第一版' })
    await vi.advanceTimersByTimeAsync(1000)
    coordinator.update({ summary: '清理期间写入' })
    cleanup.resolve()
    await vi.runAllTimersAsync()

    expect(save).toHaveBeenCalledTimes(2)
    expect(save).toHaveBeenNthCalledWith(2, 42, {
      revision: 2,
      summary: '清理期间写入',
    })
  })

  it('debounces a change queued synchronously by the final saved notification', async () => {
    const recoveryStore = createRecoveryStore()
    const save = vi
      .fn<(entryId: number, body: EntryUpdateRequest) => Promise<EntryDetail>>()
      .mockResolvedValueOnce(entryAtRevision(2))
      .mockResolvedValueOnce(entryAtRevision(3))
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })
    coordinator.subscribe((snapshot) => {
      if (snapshot.status === 'saved' && snapshot.revision === 2) {
        coordinator.update({ summary: 'saved 回调内写入' })
      }
    })

    coordinator.update({ title: '第一版' })
    await vi.advanceTimersByTimeAsync(1000)
    await vi.advanceTimersByTimeAsync(999)

    expect(save).toHaveBeenCalledOnce()
    await vi.advanceTimersByTimeAsync(1)
    expect(save).toHaveBeenCalledTimes(2)
    expect(save).toHaveBeenNthCalledWith(2, 42, {
      revision: 2,
      summary: 'saved 回调内写入',
    })
  })

  it('flushes a pending change immediately and returns the saved entry', async () => {
    const recoveryStore = createRecoveryStore()
    const savedEntry = entryAtRevision(2)
    const save = vi.fn().mockResolvedValue(savedEntry)
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })

    coordinator.update({ title: '立即保存' })
    const result = await coordinator.flush()
    await vi.advanceTimersByTimeAsync(1000)

    expect(result).toBe(savedEntry)
    expect(save).toHaveBeenCalledOnce()
  })

  it('reports an error and retains recoverable fields when cleanup fails after saving', async () => {
    const cleanupError = new Error('cleanup failed')
    const recoveryStore = createRecoveryStore({
      remove: vi.fn().mockRejectedValue(cleanupError),
    })
    const save = vi.fn().mockResolvedValue(entryAtRevision(2))
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })
    const snapshots = observe(coordinator)

    coordinator.update({ title: '已提交但未清理' })
    const result = await coordinator.flush()

    expect(result).toBeNull()
    expect(last(snapshots)).toMatchObject({
      status: 'error',
      revision: 2,
      pendingFields: { title: '已提交但未清理' },
      error: { code: 'INTERNAL', status: 0, message: 'cleanup failed' },
    })
    expect(recoveryStore.put).toHaveBeenLastCalledWith(
      expect.objectContaining({
        revision: 2,
        fields: { title: '已提交但未清理' },
        syncState: 'pending',
      }),
    )
  })

  it('settles disposal in the error state when cleanup fails', async () => {
    const cleanupError = new Error('cleanup failed')
    const recoveryStore = createRecoveryStore({
      remove: vi.fn().mockRejectedValue(cleanupError),
    })
    const save = vi.fn().mockResolvedValue(entryAtRevision(2))
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })
    const snapshots = observe(coordinator)

    coordinator.update({ title: '卸载前保存' })
    await expect(coordinator.dispose()).resolves.toBeUndefined()

    expect(last(snapshots)).toMatchObject({
      status: 'error',
      revision: 2,
      pendingFields: { title: '卸载前保存' },
    })
  })

  it('keeps every public entry point inert after disposal', async () => {
    const recoveryStore = createRecoveryStore({
      remove: vi.fn().mockRejectedValueOnce(new Error('cleanup failed')).mockResolvedValue(undefined),
    })
    const save = vi
      .fn<(entryId: number, body: EntryUpdateRequest) => Promise<EntryDetail>>()
      .mockResolvedValueOnce(entryAtRevision(2))
      .mockResolvedValueOnce(entryAtRevision(3))
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })

    coordinator.update({ title: '卸载后不可重启' })
    await coordinator.dispose()
    const lateListener = vi.fn<(snapshot: SaveSnapshot) => void>()
    coordinator.subscribe(lateListener)
    coordinator.update({ summary: '忽略此变更' })

    await expect(coordinator.flush()).resolves.toBeNull()
    await expect(coordinator.retry()).resolves.toBeNull()
    await vi.advanceTimersByTimeAsync(1000)
    expect(save).toHaveBeenCalledOnce()
    expect(lateListener).not.toHaveBeenCalled()
  })

  it('keeps pending recovery state and reports offline after a network failure', async () => {
    const recoveryStore = createRecoveryStore()
    const networkError = new ApiClientError({
      code: NETWORK_ERROR,
      status: 0,
      message: 'offline',
    })
    const save = vi.fn().mockRejectedValue(networkError)
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })
    const snapshots = observe(coordinator)

    coordinator.update({ title: '离线标题' })
    await vi.advanceTimersByTimeAsync(1000)

    expect(last(snapshots)).toEqual({
      status: 'offline',
      revision: 1,
      pendingFields: { title: '离线标题' },
      error: networkError,
    })
    expect(recoveryStore.put).toHaveBeenLastCalledWith(
      expect.objectContaining({ fields: { title: '离线标题' }, syncState: 'pending' }),
    )
    expect(recoveryStore.remove).not.toHaveBeenCalled()
  })

  it('retains changes queued while a failed save is updating recovery state', async () => {
    const failedRecoveryWrite = deferred<void>()
    const recoveryStore = createRecoveryStore({
      put: vi
        .fn<(record: RecoveryRecord) => Promise<void>>()
        .mockResolvedValueOnce(undefined)
        .mockImplementationOnce(() => failedRecoveryWrite.promise)
        .mockResolvedValueOnce(undefined),
    })
    const networkError = new ApiClientError({
      code: NETWORK_ERROR,
      status: 0,
      message: 'offline',
    })
    const save = vi.fn().mockRejectedValue(networkError)
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })
    const snapshots = observe(coordinator)

    coordinator.update({ title: '离线标题' })
    await vi.advanceTimersByTimeAsync(1000)
    coordinator.update({ content_md: '失败处理期间写入' })
    failedRecoveryWrite.resolve()
    await vi.runAllTimersAsync()

    expect(last(snapshots).pendingFields).toEqual({
      title: '离线标题',
      content_md: '失败处理期间写入',
    })
    expect(recoveryStore.put).toHaveBeenLastCalledWith(
      expect.objectContaining({
        fields: { title: '离线标题', content_md: '失败处理期间写入' },
        syncState: 'pending',
      }),
    )
  })

  it('retries retained fields with the current revision when requested', async () => {
    const recoveryStore = createRecoveryStore()
    const networkError = new ApiClientError({
      code: NETWORK_ERROR,
      status: 0,
      message: 'offline',
    })
    const savedEntry = entryAtRevision(2)
    const save = vi.fn().mockRejectedValueOnce(networkError).mockResolvedValueOnce(savedEntry)
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })
    const snapshots = observe(coordinator)

    coordinator.update({ title: '稍后重试' })
    await vi.advanceTimersByTimeAsync(1000)
    const result = await coordinator.retry()

    expect(result).toBe(savedEntry)
    expect(save).toHaveBeenCalledTimes(2)
    expect(last(snapshots)).toEqual({
      status: 'saved',
      revision: 2,
      pendingFields: null,
      error: null,
    })
  })

  it('marks recovery state as conflict and does not automatically retry a 409', async () => {
    const recoveryStore = createRecoveryStore()
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const save = vi.fn().mockRejectedValue(conflict)
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })
    const snapshots = observe(coordinator)

    coordinator.update({ title: '冲突标题' })
    await vi.advanceTimersByTimeAsync(1000)
    await vi.advanceTimersByTimeAsync(10_000)

    expect(save).toHaveBeenCalledOnce()
    expect(recoveryStore.put).toHaveBeenLastCalledWith(
      expect.objectContaining({ fields: { title: '冲突标题' }, syncState: 'conflict' }),
    )
    expect(last(snapshots)).toEqual({
      status: 'conflict',
      revision: 1,
      pendingFields: { title: '冲突标题' },
      error: conflict,
    })
  })

  it('persists edits made after a conflict without retrying the network save', async () => {
    const recoveryStore = createRecoveryStore()
    const conflict = new ApiClientError({
      code: 'CONFLICT',
      status: 409,
      message: 'revision conflict',
    })
    const save = vi.fn().mockRejectedValue(conflict)
    const coordinator = createSaveCoordinator({
      entryId: 42,
      initialRevision: 1,
      waitMs: 1000,
      recoveryStore,
      save,
    })

    coordinator.update({ title: '冲突标题', content_md: '冲突时正文' })
    await vi.advanceTimersByTimeAsync(1000)
    coordinator.update({ content_md: '409 后继续写的正文' })
    await coordinator.flush()
    await vi.advanceTimersByTimeAsync(10_000)

    expect(save).toHaveBeenCalledOnce()
    expect(recoveryStore.put).toHaveBeenLastCalledWith(
      expect.objectContaining({
        entryId: 42,
        revision: 1,
        fields: { title: '冲突标题', content_md: '409 后继续写的正文' },
        syncState: 'conflict',
      }),
    )
  })
})

interface RecoveryStoreDouble {
  put: (record: RecoveryRecord) => Promise<void>
  remove: (entryId: number) => Promise<void>
}

function createRecoveryStore(
  overrides: Partial<RecoveryStoreDouble> = {},
): RecoveryStoreDouble {
  return {
    put: vi.fn().mockResolvedValue(undefined),
    remove: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function observe(coordinator: {
  subscribe(listener: (snapshot: SaveSnapshot) => void): () => void
}): SaveSnapshot[] {
  const snapshots: SaveSnapshot[] = []
  coordinator.subscribe((snapshot) => snapshots.push(snapshot))
  return snapshots
}

function last<T>(values: T[]): T {
  return values[values.length - 1] as T
}

function deferred<T>(): { promise: Promise<T>; resolve(value: T): void } {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

function entryAtRevision(revision: number): EntryDetail {
  return {
    id: 42,
    revision,
    world: 'journal',
    kind: '',
    type: 'journal',
    title: '标题',
    slug: 'title',
    summary: '',
    content_md: '',
    cover_url: '',
    meta: {},
    word_count: 0,
    category_id: 0,
    category: null,
    happened_at: null,
    published_at: null,
    status: 'draft',
    visibility: 'private',
    created_at: '2026-08-26T00:00:00.000Z',
    updated_at: '2026-08-26T12:00:00.000Z',
  }
}
