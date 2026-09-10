import { describe, expect, it, vi } from 'vitest'
import { createMusicPlayer } from './music-player'
import { MUSIC_STORAGE_KEY } from './music'
import type { MusicCatalog } from '../types/music'

const catalog: MusicCatalog = {
  tracks: [1, 2].map((id) => ({
    id,
    title: `Song ${id}`,
    artist: '',
    audio_url: `https://audio.example/${id}.mp3`,
    cover_url: '',
    duration: 120,
    revision: 1,
  })),
  playlists: [
    {
      id: 1,
      name: 'List',
      cover_url: '',
      is_public: true,
      is_default: true,
      track_ids: [1, 2],
      revision: 1,
    },
  ],
}
class FakeAudio extends EventTarget {
  src = ''
  currentSrc = ''
  preload = ''
  currentTime = 0
  duration = 120
  volume = 1
  paused = true
  ended = false
  readyState = 0
  error: { code: number } | null = null
  requests: { resolve: () => void; reject: (error: Error) => void }[] = []
  play = vi.fn(
    () =>
      new Promise<void>((resolve, reject) => {
        this.paused = false
        this.requests.push({ resolve, reject })
      }),
  )
  pause = vi.fn(() => {
    this.paused = true
  })
  load() {
    this.currentSrc = this.src
    this.readyState = 0
    this.ended = false
    this.error = null
  }
  removeAttribute() {
    this.src = ''
    this.currentSrc = ''
  }
  emit(name: string) {
    this.dispatchEvent(new Event(name))
  }
  metadata() {
    this.readyState = 2
    this.emit('loadedmetadata')
  }
}
function setup(raw: string | null = null) {
  const audio = new FakeAudio()
  let saved = raw
  const storage = {
    getItem: vi.fn(() => saved),
    setItem: vi.fn((_key, value) => {
      saved = value
    }),
  }
  const player = createMusicPlayer(audio as unknown as HTMLAudioElement, storage)
  player.initialize(catalog)
  return { audio, player, storage }
}
const flush = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

describe('root music player', () => {
  it('restores without fetching audio or autoplay; applies position after metadata', async () => {
    const { audio, player, storage } = setup(
      JSON.stringify({ playlistId: 1, trackId: 2, position: 43, volume: 0, mode: 'single' }),
    )
    expect(audio.src).toBe('')
    expect(audio.play).not.toHaveBeenCalled()
    expect(player.state.trackId).toBe(2)
    expect(audio.volume).toBe(0)
    void player.play()
    audio.metadata()
    expect(audio.currentTime).toBe(43)
    audio.requests[0]!.resolve()
    await flush()
    player.pause()
    expect(storage.setItem).toHaveBeenCalledWith(
      MUSIC_STORAGE_KEY,
      expect.stringContaining('"position":43'),
    )
  })
  it('uses media duration when the catalog reports zero', () => {
    const { audio, player } = setup()
    player.initialize({
      ...catalog,
      tracks: catalog.tracks.map((track) => ({ ...track, duration: 0 })),
    })
    expect(player.state.duration).toBe(0)
    player.seek(20)
    expect(player.state.position).toBe(0)
    void player.play()
    audio.duration = 93
    audio.metadata()
    expect(player.state.duration).toBe(93)
    player.seek(20)
    expect(audio.currentTime).toBe(20)
  })
  it('clears buffering after an external media pause', () => {
    const { audio, player } = setup()
    void player.play()
    audio.metadata()
    audio.emit('waiting')
    expect(player.state.pending).toBe(true)
    audio.paused = true
    audio.emit('pause')
    expect(player.state.pending).toBe(false)
    expect(player.state.playing).toBe(false)
  })
  it('ignores rejected play from an old song and stale media events', async () => {
    const { audio, player } = setup()
    void player.play()
    const staleEnded = new Event('ended')
    player.select(1, 2)
    audio.metadata()
    audio.requests[0]!.reject(new Error('old source aborted'))
    audio.requests[1]!.resolve()
    await flush()
    expect(player.state.error).toBe('')
    expect(player.state.playing).toBe(true)
    audio.ended = true
    audio.dispatchEvent(staleEnded)
    expect(player.state.trackId).toBe(2)
    audio.currentSrc = catalog.tracks[0]!.audio_url
    audio.error = { code: 4 }
    audio.emit('error')
    expect(player.state.error).toBe('')
  })
  it('advances when the browser sends pause before ended at the end of a track', async () => {
    const { audio, player } = setup()
    void player.play()
    audio.metadata()
    audio.requests[0]!.resolve()
    await flush()
    audio.currentTime = audio.duration
    audio.ended = true
    audio.paused = true
    audio.emit('timeupdate')
    audio.emit('pause')
    audio.emit('ended')
    expect(player.state.trackId).toBe(2)
    expect(audio.play).toHaveBeenCalledTimes(2)
  })
  it('keeps pause authoritative over delayed play completion', async () => {
    const { audio, player } = setup()
    void player.play()
    player.pause()
    audio.paused = false
    audio.requests[0]!.resolve()
    await flush()
    expect(audio.paused).toBe(true)
    expect(player.state.playing).toBe(false)
    expect(player.state.pending).toBe(false)
  })
  it('does not pause a newer play when an older request resolves', async () => {
    const { audio, player } = setup()
    void player.play()
    player.pause()
    void player.play()
    audio.requests[1]!.resolve()
    await flush()
    audio.requests[0]!.resolve()
    await flush()
    expect(audio.paused).toBe(false)
    expect(player.state.playing).toBe(true)
  })
  it('handles metadata bounds, seek, modes and ended using the same audio', async () => {
    const { audio, player } = setup()
    void player.play()
    audio.metadata()
    player.seek(999)
    expect(audio.currentTime).toBe(120)
    player.setVolume(-4)
    expect(audio.volume).toBe(0)
    audio.ended = true
    audio.emit('ended')
    expect(player.state.trackId).toBe(2)
    player.setMode('single')
    audio.metadata()
    audio.ended = true
    audio.emit('ended')
    expect(player.state.trackId).toBe(2)
    expect(player.state.position).toBe(0)
    expect(audio.play).toHaveBeenCalledTimes(3)
    player.dispose()
    audio.requests.forEach((request) => request.resolve())
    await flush()
    expect(audio.src).toBe('')
    expect(audio.paused).toBe(true)
  })
  it('stops on broken audio without skipping, and allows explicit retry', async () => {
    const { audio, player } = setup()
    void player.play()
    audio.error = { code: 4 }
    audio.emit('error')
    audio.emit('error')
    expect(player.state.error).toContain('重试')
    expect(player.state.pending).toBe(false)
    expect(audio.play).toHaveBeenCalledTimes(1)
    expect(player.state.trackId).toBe(1)
    void player.play()
    audio.metadata()
    audio.requests[1]!.resolve()
    await flush()
    expect(player.state.error).toBe('')
    expect(player.state.playing).toBe(true)
  })
  it('reports blocked playback and safely handles denied storage and empty catalogs', async () => {
    const { audio, player } = setup()
    void player.play()
    audio.requests[0]!.reject(Object.assign(new Error(), { name: 'NotAllowedError' }))
    await flush()
    expect(player.state.error).toContain('再次点击')
    const empty = createMusicPlayer(audio as unknown as HTMLAudioElement, {
      getItem() {
        throw new Error('denied')
      },
      setItem() {
        throw new Error('denied')
      },
    })
    empty.initialize({ tracks: [], playlists: [] })
    await empty.play()
    empty.advance()
    empty.save()
    expect(empty.track.value).toBeUndefined()
    expect(empty.state.playing).toBe(false)
  })
})
