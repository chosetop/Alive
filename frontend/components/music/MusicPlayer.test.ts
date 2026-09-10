import { afterEach, describe, expect, it, vi } from 'vitest'
import { compile, createRenderer, defineComponent, h, nextTick, ref } from 'vue'
import appSource from '../../app.vue?raw'
import { useMusicPlayer } from '../../composables/useMusicPlayer'

const template = appSource.match(/<template>([\s\S]*)<\/template>/)![1]!
const App = defineComponent({ render: compile(template) })
// The real composable is mounted in the actual app template; only its view is simplified.
const MusicPlayer = defineComponent({
  setup() {
    const { player } = useMusicPlayer()
    return () =>
      h('aside', { 'aria-label': '背景音乐' }, [
        h('button', { 'aria-label': '播放', onClick: () => player.value?.play() }),
      ])
  },
})

// Exercise actual root/component lifecycle without adding a DOM dependency.
interface HostNode {
  type: string
  children: HostNode[]
  parent: HostNode | null
  props: Record<string, unknown>
  text: string
}
function node(type: string, text = ''): HostNode {
  return { type, children: [], parent: null, props: {}, text }
}
const renderer = createRenderer<HostNode, HostNode>({
  createElement: (type) => node(type),
  createText: (text) => node('text', text),
  createComment: (text) => node('comment', text),
  setText: (target, text) => {
    target.text = text
  },
  setElementText: (target, text) => {
    target.text = text
    target.children = []
  },
  patchProp: (target, key, _old, value) => {
    target.props[key] = value
  },
  parentNode: (target) => target.parent,
  nextSibling: (target) =>
    target.parent?.children[target.parent.children.indexOf(target) + 1] ?? null,
  insert(target, parent, anchor = null) {
    if (target.parent) target.parent.children.splice(target.parent.children.indexOf(target), 1)
    const index = anchor ? parent.children.indexOf(anchor) : -1
    parent.children.splice(index < 0 ? parent.children.length : index, 0, target)
    target.parent = parent
  },
  remove(target) {
    target.parent?.children.splice(target.parent.children.indexOf(target), 1)
    target.parent = null
  },
})
function find(root: HostNode, predicate: (target: HostNode) => boolean): HostNode | undefined {
  if (predicate(root)) return root
  for (const child of root.children) {
    const result = find(child, predicate)
    if (result) return result
  }
}
const flush = async () => {
  await Promise.resolve()
  await nextTick()
  await Promise.resolve()
  await nextTick()
}
afterEach(() => vi.unstubAllGlobals())

describe('music root integration', () => {
  it('fetches once and preserves the same audio and playback across route replacements', async () => {
    const audio = Object.assign(new EventTarget(), {
      preload: '',
      src: '',
      currentSrc: '',
      volume: 1,
      paused: true,
      pause: vi.fn(() => {
        audio.paused = true
      }),
      load: vi.fn(),
      removeAttribute: vi.fn(() => {
        audio.src = ''
      }),
      play: vi.fn(async () => {
        audio.paused = false
      }),
    })
    const audioFactory = vi.fn(function () {
      return audio
    })
    const get = vi.fn(async () => ({
      tracks: [
        {
          id: 1,
          title: 'Synthetic song',
          artist: 'Test artist',
          audio_url: 'https://example.test/a.wav',
          cover_url: '',
          duration: 0,
          revision: 1,
        },
      ],
      playlists: [
        {
          id: 1,
          name: 'Test list',
          cover_url: '',
          is_default: true,
          is_public: true,
          track_ids: [1],
          revision: 1,
        },
      ],
    }))
    vi.stubGlobal('Audio', audioFactory)
    vi.stubGlobal('useApi', () => ({ get }))
    vi.stubGlobal(
      'window',
      Object.assign(new EventTarget(), { localStorage: { getItem: () => null, setItem: vi.fn() } }),
    )
    vi.stubGlobal('document', new EventTarget())
    const route = ref('home')
    const slot = defineComponent({
      setup:
        (_, { slots }) =>
        () =>
          slots.default?.(),
    })
    const page = defineComponent({
      setup: () => () => h('main', { key: route.value }, route.value),
    })
    const app = renderer.createApp(App)
    app
      .component('NuxtLayout', slot)
      .component('NuxtPage', page)
      .component('ClientOnly', slot)
      .component('MusicPlayer', MusicPlayer)
    const root = node('root')
    app.mount(root)
    await flush()
    expect(get).toHaveBeenCalledExactlyOnceWith('/music')
    expect(audioFactory).toHaveBeenCalledTimes(1)
    expect(audio.src).toBe('')
    expect(audio.play).not.toHaveBeenCalled()
    const widget = find(root, (target) => target.props['aria-label'] === '背景音乐')
    expect(widget).toBeDefined()
    const button = find(root, (target) => target.props['aria-label'] === '播放')!
    ;(button.props.onClick as () => void)()
    await flush()
    expect(audio.play).toHaveBeenCalledTimes(1)
    expect(audio.paused).toBe(false)
    route.value = 'journal'
    await flush()
    expect(find(root, (target) => target.type === 'main')?.text).toBe('journal')
    expect(find(root, (target) => target.props['aria-label'] === '背景音乐')).toBe(widget)
    expect(audioFactory).toHaveBeenCalledTimes(1)
    expect(audio.paused).toBe(false)
    expect(get).toHaveBeenCalledTimes(1)
    app.unmount()
    expect(audio.paused).toBe(true)
    expect(audio.src).toBe('')
  })
})
