import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import type { MusicCatalog } from '~/types/music'
import { createMusicPlayer, type MusicPlayer } from '~/utils/music-player'

/** Called only by the player mounted outside NuxtPage, never by route components. */
export function useMusicPlayer() {
  const api = useApi()
  const player = shallowRef<MusicPlayer>()
  const loading = ref(true)
  const error = ref('')
  let disposed = false
  async function load() {
    loading.value = true
    error.value = ''
    try {
      const catalog = await api.get<MusicCatalog>('/music')
      if (!disposed) player.value?.initialize(catalog)
    } catch {
      if (!disposed) error.value = '音乐暂时未能加载'
    } finally {
      if (!disposed) loading.value = false
    }
  }
  const save = () => player.value?.save()
  onMounted(() => {
    let storage: Storage | undefined
    try {
      storage = window.localStorage
    } catch {
      /* Private browsing. */
    }
    player.value = createMusicPlayer(new Audio(), storage)
    window.addEventListener('pagehide', save)
    void load()
  })
  onBeforeUnmount(() => {
    disposed = true
    window.removeEventListener('pagehide', save)
    player.value?.dispose()
  })
  return { player, loading, error, reload: load }
}
