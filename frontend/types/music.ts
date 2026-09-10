export interface MusicTrack {
  id: number
  title: string
  artist: string
  audio_url: string
  cover_url: string
  duration: number
  revision: number
}

export interface MusicPlaylist {
  id: number
  name: string
  cover_url: string
  is_public: boolean
  is_default: boolean
  track_ids: number[]
  revision: number
}

export interface MusicCatalog {
  tracks: MusicTrack[]
  playlists: MusicPlaylist[]
}

export type MusicMode = 'list' | 'single' | 'shuffle'
export interface MusicPreference {
  playlistId: number | null
  trackId: number | null
  position: number
  volume: number
  mode: MusicMode
}
