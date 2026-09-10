import type { MusicCatalog, MusicMode, MusicPreference } from '../types/music'

export const MUSIC_STORAGE_KEY = 'alive:music:v1'
export const musicModes: MusicMode[] = ['list', 'single', 'shuffle']
export const musicModeLabels: Record<MusicMode, string> = {
  list: '列表循环',
  single: '单曲循环',
  shuffle: '随机播放',
}
export function bounded(value: unknown, min: number, max: number, fallback = min): number {
  return typeof value === 'number' && Number.isFinite(value)
    ? Math.min(max, Math.max(min, value))
    : fallback
}

export function publicCatalog(catalog: MusicCatalog): MusicCatalog {
  const tracks = catalog.tracks.filter(
    (track) => track.audio_url && /^https?:\/\//i.test(track.audio_url),
  )
  const ids = new Set(tracks.map((track) => track.id))
  const playlists = catalog.playlists
    .filter((playlist) => playlist.is_public)
    .map((playlist) => ({
      ...playlist,
      track_ids: [...new Set(playlist.track_ids.filter((id) => ids.has(id)))],
    }))
    .filter((playlist) => playlist.track_ids.length)
    .sort((a, b) => Number(b.is_default) - Number(a.is_default) || a.id - b.id)
  const publicIds = new Set(playlists.flatMap((playlist) => playlist.track_ids))
  return { tracks: tracks.filter((track) => publicIds.has(track.id)), playlists }
}

export function restoreMusic(raw: string | null, catalog: MusicCatalog): MusicPreference {
  let saved: Partial<MusicPreference> = {}
  try {
    const parsed: unknown = JSON.parse(raw || '{}')
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) saved = parsed
  } catch {
    /* Corrupt or unavailable storage should never prevent listening. */
  }
  const playlist =
    catalog.playlists.find((item) => item.id === saved.playlistId) || catalog.playlists[0]
  const validTrack = playlist?.track_ids.includes(saved.trackId as number)
  return {
    playlistId: playlist?.id ?? null,
    trackId: validTrack ? saved.trackId! : (playlist?.track_ids[0] ?? null),
    position: validTrack ? bounded(saved.position, 0, 86400) : 0,
    volume: bounded(saved.volume, 0, 1, 0.7),
    mode: musicModes.includes(saved.mode as MusicMode) ? saved.mode! : 'list',
  }
}

export function nextMusicTrack(
  ids: number[],
  current: number | null,
  mode: MusicMode,
  direction = 1,
  ended = false,
  random = Math.random,
): number | null {
  if (!ids.length) return null
  const index = ids.indexOf(current as number)
  if (ended && mode === 'single' && index >= 0) return current
  if (mode === 'shuffle' && ids.length > 1) {
    const alternatives = ids.filter((id) => id !== current)
    return alternatives[Math.floor(bounded(random(), 0, 0.999999) * alternatives.length)]!
  }
  return ids[(Math.max(0, index) + direction + ids.length) % ids.length]!
}

export function musicTime(seconds: number): string {
  const value = Math.floor(bounded(seconds, 0, 86400))
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`
}
