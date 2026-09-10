import { describe, expect, it } from 'vitest'
import { musicTime, nextMusicTrack, publicCatalog, restoreMusic } from './music'
import type { MusicCatalog } from '../types/music'

export const catalog: MusicCatalog = {
  tracks: [1, 2, 3].map((id) => ({
    id,
    title: `Song ${id}`,
    artist: 'Artist',
    audio_url: `https://audio.example/${id}.mp3`,
    cover_url: '',
    duration: 120,
    revision: 1,
  })),
  playlists: [
    {
      id: 1,
      name: 'First',
      cover_url: '',
      is_public: true,
      is_default: false,
      track_ids: [1, 2],
      revision: 1,
    },
    {
      id: 2,
      name: 'Default',
      cover_url: '',
      is_public: true,
      is_default: true,
      track_ids: [2, 3],
      revision: 1,
    },
  ],
}
describe('music preferences and queue', () => {
  it('prioritizes default and removes private, empty and missing references', () => {
    const result = publicCatalog({
      ...catalog,
      playlists: [
        { ...catalog.playlists[0]!, is_public: false },
        { ...catalog.playlists[1]!, track_ids: [2, 2, 999] },
        { ...catalog.playlists[0]!, id: 3, track_ids: [] },
      ],
    })
    expect(result.playlists.map((item) => item.id)).toEqual([2])
    expect(result.playlists[0]?.track_ids).toEqual([2])
    expect(result.tracks.map((item) => item.id)).toEqual([2])
  })
  it('restores only track membership, clamps volume, and defaults corrupt fields', () => {
    const source = publicCatalog(catalog)
    expect(restoreMusic('{broken', source)).toEqual({
      playlistId: 2,
      trackId: 2,
      position: 0,
      volume: 0.7,
      mode: 'list',
    })
    expect(
      restoreMusic(
        JSON.stringify({ playlistId: 1, trackId: 3, position: 99, volume: 4, mode: 'bad' }),
        source,
      ),
    ).toEqual({ playlistId: 1, trackId: 1, position: 0, volume: 1, mode: 'list' })
    expect(
      restoreMusic(
        JSON.stringify({ playlistId: 1, trackId: 2, position: 45, volume: 0, mode: 'single' }),
        source,
      ),
    ).toEqual({ playlistId: 1, trackId: 2, position: 45, volume: 0, mode: 'single' })
    for (const raw of ['null', '[]', '1', '{"position":"10","volume":"1"}'])
      expect(restoreMusic(raw, source).volume).toBe(0.7)
    expect(restoreMusic(null, { tracks: [], playlists: [] }).trackId).toBeNull()
  })
  it('wraps list, repeats ended single only, and shuffles without immediate repeats', () => {
    expect(nextMusicTrack([], null, 'list')).toBeNull()
    expect(nextMusicTrack([1, 2], 2, 'list', 1, true)).toBe(1)
    expect(nextMusicTrack([1, 2], 1, 'list', -1)).toBe(2)
    expect(nextMusicTrack([1, 2], 1, 'single', 1, true)).toBe(1)
    expect(nextMusicTrack([1, 2], 1, 'single')).toBe(2)
    expect(nextMusicTrack([1, 2, 3], 2, 'shuffle', 1, true, () => 0)).toBe(1)
    expect(nextMusicTrack([1, 2, 3], 2, 'shuffle', 1, true, () => 0.99)).toBe(3)
    expect(nextMusicTrack([1], 1, 'shuffle')).toBe(1)
    expect(musicTime(Infinity)).toBe('0:00')
    expect(musicTime(125)).toBe('2:05')
  })
})
