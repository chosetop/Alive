import type { WorldKey } from '../types/api'

/** Stable fallback for entries whose editor did not receive a manual slug. */
export function defaultEntrySlug(world: WorldKey, id: number): string {
  return `${world}-${id}`
}
