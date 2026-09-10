import { computed, reactive } from 'vue'
import type { MusicCatalog, MusicMode, MusicPreference } from '../types/music'
import { bounded, MUSIC_STORAGE_KEY, nextMusicTrack, publicCatalog, restoreMusic } from './music'

type StoragePort = Pick<Storage, 'getItem' | 'setItem'>

/** One root-owned media element. Injected ports keep race and storage tests real. */
export function createMusicPlayer(audio: HTMLAudioElement, storage?: StoragePort) {
  const state = reactive({
    catalog: { tracks: [], playlists: [] } as MusicCatalog,
    playlistId: null as number | null,
    trackId: null as number | null,
    position: 0,
    duration: 0,
    volume: 0.7,
    mode: 'list' as MusicMode,
    playing: false,
    pending: false,
    error: '',
  })
  const track = computed(() => state.catalog.tracks.find((item) => item.id === state.trackId))
  const playlist = computed(() =>
    state.catalog.playlists.find((item) => item.id === state.playlistId),
  )
  const songs = computed(() =>
    (playlist.value?.track_ids ?? []).flatMap((id) => {
      const song = state.catalog.tracks.find((item) => item.id === id)
      return song ? [song] : []
    }),
  )
  let generation = 0
  let playRequest = 0
  let wanted = false
  let disposed = false
  let loadedId: number | null = null
  let removeEvents = () => {}
  let lastSavedSecond = -1
  audio.preload = 'none'

  function save() {
    const preference: MusicPreference = {
      playlistId: state.playlistId,
      trackId: state.trackId,
      position: state.position,
      volume: state.volume,
      mode: state.mode,
    }
    try {
      storage?.setItem(MUSIC_STORAGE_KEY, JSON.stringify(preference))
    } catch {
      /* Private mode/quota. */
    }
  }
  function pause(persist = true) {
    wanted = false
    playRequest++
    audio.pause()
    state.playing = false
    state.pending = false
    if (persist) save()
  }
  function fail(message: string) {
    pause()
    state.error = message
    // Deliberately no error-skipping: an all-broken playlist must terminate.
  }
  function clearSource() {
    generation++
    removeEvents()
    pause(false)
    loadedId = null
    audio.removeAttribute('src')
    audio.load()
  }
  function initialize(catalog: MusicCatalog) {
    clearSource()
    state.catalog = publicCatalog(catalog)
    let raw: string | null = null
    try {
      raw = storage?.getItem(MUSIC_STORAGE_KEY) ?? null
    } catch {
      /* Storage denied. */
    }
    Object.assign(state, restoreMusic(raw, state.catalog))
    state.duration = 0
    state.error = ''
    audio.volume = state.volume
  }
  function loadSource() {
    if (!track.value) return
    const token = ++generation
    const since = performance.now()
    const expected = new URL(track.value.audio_url).href
    removeEvents()
    audio.pause()
    const listeners: [string, EventListener][] = []
    const listen = (name: string, callback: () => void) => {
      const listener: EventListener = (event) => {
        // Reject old queued events as well as callbacks from a replaced source.
        if (
          disposed ||
          token !== generation ||
          event.timeStamp < since ||
          audio.currentSrc !== expected
        )
          return
        callback()
      }
      audio.addEventListener(name, listener)
      listeners.push([name, listener])
    }
    removeEvents = () =>
      listeners.forEach(([name, listener]) => audio.removeEventListener(name, listener))
    listen('loadedmetadata', () => {
      if (audio.readyState < 1) return
      state.duration = bounded(audio.duration, 0, 86400)
      state.position = bounded(state.position, 0, state.duration)
      // A stored end position starts from the beginning on explicit playback.
      if (state.position >= state.duration) state.position = 0
      audio.currentTime = state.position
    })
    listen('timeupdate', () => {
      if (audio.readyState < 1) return
      state.position = bounded(audio.currentTime, 0, state.duration)
      const second = Math.floor(state.position)
      if (second !== lastSavedSecond) {
        lastSavedSecond = second
        save()
      }
    })
    listen('playing', () => {
      if (!wanted) {
        audio.pause()
        return
      }
      if (audio.paused || audio.readyState < 2) return
      state.playing = true
      state.pending = false
      state.error = ''
    })
    listen('pause', () => {
      // Browsers pause at natural end before dispatching ended. Preserve intent
      // until ended advances; explicit pause() already clears wanted itself.
      if (!audio.paused || audio.ended) return
      state.playing = false
      state.pending = false
      wanted = false
      playRequest++
      save()
    })
    listen('waiting', () => {
      if (wanted && !audio.paused) state.pending = true
    })
    listen('ended', () => {
      if (!wanted || !audio.ended || audio.readyState < 2) return
      advance(1, true)
    })
    listen('error', () => {
      if (audio.error) fail('这首歌暂时无法播放，请重试或选择其他歌曲。')
    })
    loadedId = state.trackId
    audio.src = expected
    audio.load()
  }
  async function play() {
    if (disposed || !track.value) return
    wanted = true
    state.pending = true
    if (loadedId !== state.trackId || state.error) loadSource()
    state.error = ''
    const request = ++playRequest
    const token = generation
    try {
      await audio.play()
      if (disposed || !wanted) {
        audio.pause()
        return
      }
      if (disposed || token !== generation || request !== playRequest) return
      if (!wanted) {
        audio.pause()
        return
      }
      state.playing = !audio.paused
      state.pending = false
    } catch (cause) {
      if (disposed || token !== generation || request !== playRequest) return
      const blocked = cause instanceof Error && cause.name === 'NotAllowedError'
      fail(blocked ? '浏览器尚未允许播放，请再次点击播放。' : '播放失败，请重试或选择其他歌曲。')
    }
  }
  function select(playlistId: number, trackId?: number, start = true) {
    const list = state.catalog.playlists.find((item) => item.id === playlistId)
    const id = trackId ?? list?.track_ids[0]
    if (!list || !id || !list.track_ids.includes(id)) return
    clearSource()
    state.playlistId = playlistId
    state.trackId = id
    state.position = 0
    state.duration = 0
    state.error = ''
    save()
    if (start) void play()
  }
  function advance(direction = 1, ended = false) {
    const id = nextMusicTrack(
      playlist.value?.track_ids ?? [],
      state.trackId,
      state.mode,
      direction,
      ended,
    )
    if (id !== null && state.playlistId !== null) select(state.playlistId, id)
  }
  function seek(value: number) {
    if (!state.duration || audio.readyState < 1) return
    state.position = bounded(value, 0, state.duration)
    audio.currentTime = state.position
    save()
  }
  function setVolume(value: number) {
    state.volume = bounded(value, 0, 1, state.volume)
    audio.volume = state.volume
    save()
  }
  function setMode(value: MusicMode) {
    state.mode = value
    save()
  }
  function dispose() {
    save()
    clearSource()
    disposed = true
  }
  return {
    state,
    track,
    playlist,
    songs,
    initialize,
    play,
    pause,
    select,
    advance,
    seek,
    setVolume,
    setMode,
    save,
    dispose,
  }
}
export type MusicPlayer = ReturnType<typeof createMusicPlayer>
