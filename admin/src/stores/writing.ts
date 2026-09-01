import { defineStore } from 'pinia'
import { ref, type InjectionKey, type Ref } from 'vue'
import type { EntryListItem, WorldKey } from '../types/api'

/**
 * How the directory asks the open editor to flush before it navigates away.
 *
 * Provide/inject rather than store state, and it lives here only because this is
 * the module both sides already import. The directory and the editor are
 * siblings under the writing layout, so neither can hand the other a prop; but
 * the callback is a live reference to a mounted component's coordinator, and
 * putting that in a Pinia store would make a function with a lifetime look like
 * a serialisable value with none.
 *
 * The ref holds null whenever no editor is mounted, which is a real state: the
 * mobile drawer can be open over an empty canvas.
 */
export type WritingFlushGate = Ref<(() => Promise<void>) | null>

export const writingFlushKey: InjectionKey<WritingFlushGate> = Symbol('writing-flush')

/**
 * Shell state for the writing workspace.
 *
 * The scope here is deliberately narrow: which chrome is visible, which article
 * the chrome should point at, and what is typed in the directory's search box.
 * Nothing about the article itself lives here.
 *
 * That boundary is the whole reason this store exists rather than being folded
 * into the editor view. Document content belongs to Milkdown and reaches the
 * server through the save coordinator; putting either behind a global store
 * would give two owners to one value, and the losing writer is whichever one
 * ran second. Text quietly overwritten that way is not a bug anybody can see in
 * the UI, so the guard has to be structural.
 *
 * `directoryOpen` starts true. A first-time visit that opened onto a bare canvas
 * would hide the only route to the rest of the articles, and the collapse is
 * cheap to reach once you know it is there.
 */
export const useWritingStore = defineStore('writing', () => {
  const directoryOpen = ref(true)
  /**
   * The article the workspace is currently on.
   *
   * Set by the editor rather than read from the route, because `/entries/new`
   * has no id in its path: the blank draft is created first and the route is
   * replaced afterwards. Reading the route would leave the directory with
   * nothing highlighted for exactly as long as that round trip takes.
   */
  const activeEntryId = ref<number | null>(null)
  const activeWorld = ref<WorldKey | null>(null)
  const searchQuery = ref('')
  const directoryEntries = ref<EntryListItem[]>([])

  function toggleDirectory(): void {
    directoryOpen.value = !directoryOpen.value
  }

  function setDirectoryOpen(open: boolean): void {
    directoryOpen.value = open
  }

  function setActiveEntry(entryId: number | null): void {
    activeEntryId.value = entryId
  }

  function setActiveWorld(world: WorldKey | null): void {
    if (activeWorld.value === world) return
    activeWorld.value = world
    activeEntryId.value = null
    searchQuery.value = ''
    directoryEntries.value = []
  }

  function setSearchQuery(query: string): void {
    searchQuery.value = query
  }

  function setDirectoryEntries(entries: EntryListItem[]): void {
    directoryEntries.value = entries
  }

  function nextEntryAfterDelete(entryId: number): number | null {
    const index = directoryEntries.value.findIndex((entry) => entry.id === entryId)
    if (index === -1) return null
    return directoryEntries.value[index + 1]?.id ?? directoryEntries.value[index - 1]?.id ?? null
  }

  function removeDirectoryEntry(entryId: number): void {
    directoryEntries.value = directoryEntries.value.filter((entry) => entry.id !== entryId)
  }

  return {
    directoryOpen,
    activeEntryId,
    activeWorld,
    searchQuery,
    directoryEntries,
    toggleDirectory,
    setDirectoryOpen,
    setActiveEntry,
    setActiveWorld,
    setSearchQuery,
    setDirectoryEntries,
    nextEntryAfterDelete,
    removeDirectoryEntry,
  }
})
