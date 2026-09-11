<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useMusicPlayer } from '~/composables/useMusicPlayer'
import { musicTime } from '~/utils/music'

const { player, loading, error, reload } = useMusicPlayer()
const expanded = ref(false)
const root = ref<HTMLElement>()
const trigger = ref<HTMLButtonElement>()
const panel = ref<HTMLElement>()
const isPlaying = computed(() =>
  Boolean(player.value?.state.playing || player.value?.state.pending),
)
const progress = computed(() => {
  const duration = player.value?.state.duration || player.value?.track.value?.duration || 0
  return duration
    ? Math.min(100, Math.max(0, ((player.value?.state.position || 0) / duration) * 100))
    : 0
})

async function toggle() {
  expanded.value = !expanded.value
  if (expanded.value) {
    await nextTick()
    panel.value?.querySelector<HTMLButtonElement>('button')?.focus()
  }
}
function close(restore = true) {
  expanded.value = false
  if (restore) trigger.value?.focus()
}
function togglePlayback() {
  if (!player.value) return
  if (isPlaying.value) player.value.pause()
  else void player.value.play()
}
function toggleShuffle() {
  if (!player.value) return
  player.value.setMode(player.value.state.mode === 'shuffle' ? 'list' : 'shuffle')
}
function toggleRepeat() {
  if (!player.value) return
  player.value.setMode(player.value.state.mode === 'single' ? 'list' : 'single')
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && expanded.value) {
    event.preventDefault()
    close()
  }
}
function outside(event: PointerEvent) {
  if (event.target instanceof Node && !root.value?.contains(event.target)) close(false)
}
function value(event: Event) {
  return Number((event.target as HTMLInputElement).value)
}
function selectPlaylist(id: string) {
  player.value?.select(Number(id), undefined, false)
}
onMounted(() => {
  document.addEventListener('keydown', keydown)
  document.addEventListener('pointerdown', outside)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', keydown)
  document.removeEventListener('pointerdown', outside)
})
</script>

<template>
  <aside
    v-if="loading || error || player?.track.value"
    ref="root"
    class="music-player"
    :class="{ 'is-expanded': expanded }"
    aria-label="背景音乐"
  >
    <div v-if="loading || error" class="music-notice" role="status">
      <span>{{ loading ? '正在加载音乐…' : error }}</span>
      <button v-if="error" type="button" @click="reload">重试</button>
    </div>

    <template v-else-if="player && player.track.value">
      <Transition name="music-expand">
        <section
          v-if="expanded"
          id="music-panel"
          ref="panel"
          class="music-panel"
          aria-label="歌曲与播放设置"
        >
          <header class="panel-heading">
            <h2>此刻听见</h2>
            <button type="button" aria-label="收起音乐播放器" @click="close()">
              收起
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m7 14 5-5 5 5" /></svg>
            </button>
          </header>

          <div class="now-playing">
            <span class="now-cover">
              <img
                v-if="player.track.value.cover_url || player.playlist.value?.cover_url"
                :key="player.track.value.cover_url || player.playlist.value?.cover_url"
                :src="player.track.value.cover_url || player.playlist.value?.cover_url"
                alt=""
                @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
              />
              <span v-else aria-hidden="true">♪</span>
            </span>
            <div class="now-copy">
              <span class="now-kicker">正在播放</span>
              <strong>{{ player.track.value.title }}</strong>
              <span class="now-artist">{{ player.track.value.artist || '背景音乐' }}</span>
              <UiSelect
                class="playlist-select"
                :model-value="String(player.state.playlistId ?? '')"
                :options="player.state.catalog.playlists.map((list) => ({ value: String(list.id), label: list.name }))"
                label="选择歌单"
                size="compact"
                @change="selectPlaylist"
              />
            </div>
            <div class="volume-control">
              <label for="music-volume" aria-label="音量">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M5 9v6h4l5 4V5L9 9H5ZM18 9a4 4 0 0 1 0 6" />
                </svg>
              </label>
              <input
                id="music-volume"
                type="range"
                min="0"
                max="1"
                step="0.01"
                :value="player.state.volume"
                :aria-valuetext="`${Math.round(player.state.volume * 100)}%`"
                @input="player.setVolume(value($event))"
              />
            </div>
          </div>

          <div class="seek-row">
            <span>{{ musicTime(player.state.position) }}</span>
            <label class="sr-only" for="music-seek">播放进度</label>
            <input
              id="music-seek"
              type="range"
              min="0"
              :max="player.state.duration || 0"
              step="0.1"
              :value="player.state.position"
              :disabled="!player.state.duration"
              :aria-valuetext="musicTime(player.state.position)"
              @input="player.seek(value($event))"
            />
            <span>{{ musicTime(player.state.duration || player.track.value.duration) }}</span>
          </div>

          <div class="playback-row">
            <div class="panel-transport">
              <button
                type="button"
                :class="{ active: player.state.mode === 'shuffle' }"
                :aria-pressed="player.state.mode === 'shuffle'"
                :aria-label="player.state.mode === 'shuffle' ? '关闭随机播放' : '随机播放'"
                title="随机播放"
                @click="toggleShuffle"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path
                    d="M4 7h3c5 0 5 10 10 10h3M17 4l3 3-3 3M4 17h3c2.2 0 3.4-1.9 4.5-4M17 14l3 3-3 3"
                  />
                </svg>
              </button>
              <button type="button" aria-label="上一首" @click="player.advance(-1)">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M6 5v14M19 5 8 12l11 7Z" />
                </svg>
              </button>
              <button
                type="button"
                class="panel-play"
                :aria-label="isPlaying ? '暂停' : '播放'"
                @click="togglePlayback"
              >
                <svg v-if="isPlaying" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M8 5v14M16 5v14" />
                </svg>
                <svg v-else viewBox="0 0 24 24" aria-hidden="true">
                  <path d="m8 4 12 8-12 8Z" />
                </svg>
              </button>
              <button type="button" aria-label="下一首" @click="player.advance()">
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M18 5v14M5 5l11 7-11 7Z" />
                </svg>
              </button>
              <button
                type="button"
                :class="{ active: player.state.mode === 'single' }"
                :aria-pressed="player.state.mode === 'single'"
                :aria-label="player.state.mode === 'single' ? '关闭单曲循环' : '单曲循环'"
                title="单曲循环"
                @click="toggleRepeat"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path
                    d="M17 2l3 3-3 3M20 5H8a4 4 0 0 0-4 4v1M7 22l-3-3 3-3M4 19h12a4 4 0 0 0 4-4v-1"
                  />
                  <text v-if="player.state.mode === 'single'" x="10" y="15">1</text>
                </svg>
              </button>
            </div>
          </div>

          <div class="queue-heading">
            <h3>接下来</h3>
            <span>{{ player.songs.value.length }} 首</span>
          </div>
          <ol class="song-list" aria-label="歌曲队列">
            <li v-for="(song, index) in player.songs.value" :key="song.id">
              <button
                type="button"
                :aria-current="song.id === player.state.trackId ? 'true' : undefined"
                @click="player.select(player.state.playlistId!, song.id)"
              >
                <span class="song-number">{{ String(index + 1).padStart(2, '0') }}</span>
                <span v-if="song.id === player.state.trackId" class="equalizer" aria-hidden="true">
                  <i></i><i></i><i></i>
                </span>
                <span class="song-text">
                  <span>{{ song.title }}</span>
                  <small>{{ song.artist || '未知歌手' }}</small>
                </span>
                <span class="song-duration">{{ musicTime(song.duration) }}</span>
              </button>
            </li>
          </ol>
        </section>
      </Transition>

      <div v-if="player.state.error" class="music-error" role="status">
        {{ player.state.error }} <button type="button" @click="player.play()">重试</button>
      </div>

      <div class="music-bar">
        <button
          ref="trigger"
          class="track-trigger"
          type="button"
          :aria-expanded="expanded"
          aria-controls="music-panel"
          :aria-label="`${player.track.value.title}，${expanded ? '收起' : '展开'}歌单与播放设置`"
          @click="toggle"
        >
          <span class="cover">
            <img
              v-if="player.track.value.cover_url || player.playlist.value?.cover_url"
              :key="player.track.value.cover_url || player.playlist.value?.cover_url"
              :src="player.track.value.cover_url || player.playlist.value?.cover_url"
              alt=""
              @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
            />
            <span v-else aria-hidden="true">♪</span>
          </span>
          <span class="track-text">
            <span>{{ player.track.value.title }}</span>
            <small>{{
              player.state.pending ? '正在缓冲…' : player.track.value.artist || '背景音乐'
            }}</small>
          </span>
          <span class="expand-mark" aria-hidden="true">
            <svg viewBox="0 0 24 24">
              <path :d="expanded ? 'm7 14 5-5 5 5' : 'm7 10 5 5 5-5'" />
            </svg>
          </span>
        </button>

        <div v-if="!expanded" class="bar-progress" aria-hidden="true">
          <span :style="{ width: `${progress}%` }"></span>
        </div>

        <div v-if="!expanded" class="transport">
          <button type="button" aria-label="上一首" @click="player.advance(-1)">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 5v14M19 5 8 12l11 7Z" /></svg>
          </button>
          <button
            type="button"
            class="play-button"
            :aria-label="isPlaying ? '暂停' : '播放'"
            @click="togglePlayback"
          >
            <svg v-if="isPlaying" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M8 5v14M16 5v14" />
            </svg>
            <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="m8 4 12 8-12 8Z" /></svg>
          </button>
          <button type="button" aria-label="下一首" @click="player.advance()">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18 5v14M5 5l11 7-11 7Z" /></svg>
          </button>
        </div>
      </div>
    </template>
  </aside>
</template>

<style scoped>
.music-player {
  --music-glass-fallback: color-mix(in srgb, var(--c-surface-sunken) 82%, var(--c-paper) 18%);
  --music-glass-bar: color-mix(in srgb, var(--c-surface-sunken) 76%, transparent);
  --music-glass-panel: color-mix(in srgb, var(--c-surface-sunken) 84%, transparent);
  --music-glass-border: color-mix(in srgb, var(--c-ink) 13%, transparent);
  --music-glass-highlight: color-mix(in srgb, white 24%, transparent);
  --music-glass-blur-bar: 20px;
  --music-glass-blur-panel: 28px;
  --music-glass-shadow-bar: 0 0.75rem 2rem color-mix(in srgb, var(--c-shadow-float) 72%, transparent);
  --music-glass-shadow-panel: 0 1.25rem 3.5rem color-mix(in srgb, var(--c-shadow-float) 86%, transparent);
  position: fixed;
  left: 50%;
  bottom: max(1rem, env(safe-area-inset-bottom));
  z-index: 50;
  width: min(47.5rem, calc(100vw - 2rem));
  color: var(--c-ink);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  line-height: 1.45;
  transform: translateX(-50%);
}
.music-bar,
.music-panel,
.music-notice,
.music-error {
  border: 1px solid var(--music-glass-border);
  background: var(--music-glass-fallback);
  box-shadow:
    inset 0 1px 0 var(--music-glass-highlight),
    var(--music-glass-shadow-bar);
}
.music-bar,
.music-panel {
  border-color: var(--c-glass-border);
}
.music-bar {
  background: var(--music-glass-bar);
  -webkit-backdrop-filter: blur(var(--music-glass-blur-bar)) saturate(155%);
  backdrop-filter: blur(var(--music-glass-blur-bar)) saturate(155%);
  box-shadow:
    inset 0 1px 0 var(--music-glass-highlight),
    var(--music-glass-shadow-bar);
}
.music-panel {
  background: var(--music-glass-panel);
  -webkit-backdrop-filter: blur(var(--music-glass-blur-panel)) saturate(155%);
  backdrop-filter: blur(var(--music-glass-blur-panel)) saturate(155%);
  box-shadow:
    inset 0 1px 0 var(--music-glass-highlight),
    var(--music-glass-shadow-panel);
}
.music-panel {
  position: relative;
  max-height: calc(100dvh - 8.5rem);
  margin-bottom: var(--space-3);
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: var(--space-5) var(--space-6) var(--space-4);
  border-radius: 1.5rem;
}
.music-bar {
  position: relative;
  display: grid;
  grid-template-columns: minmax(13rem, 1fr) minmax(8rem, 1.4fr) auto;
  align-items: center;
  gap: var(--space-4);
  min-height: 4.75rem;
  padding: 0.625rem 0.75rem;
  border-radius: 1.25rem;
  transition: opacity var(--duration-mid) var(--ease-out);
}
.music-bar::before,
.music-panel::before {
  position: absolute;
  pointer-events: none;
  content: '';
  border-radius: inherit;
  background: linear-gradient(112deg, color-mix(in srgb, white 22%, transparent), transparent 34%);
  opacity: 0.72;
}
.music-bar::before {
  inset: 1px;
}
.music-panel::before {
  inset: 1px;
  height: 3.5rem;
  border-radius: 1.4rem 1.4rem 0 0;
}
.music-panel > * {
  position: relative;
  z-index: 1;
}
.is-expanded .music-bar {
  grid-template-columns: 1fr;
  min-height: 4rem;
  opacity: 0.82;
}
button {
  border: 0;
  border-radius: 0.75rem;
  background: transparent;
  cursor: pointer;
  transition:
    color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out),
    transform var(--duration-fast) var(--ease-out);
}
button:hover {
  background: var(--c-surface-sunken);
}
button:active {
  transform: scale(0.96);
}
button:focus-visible,
input:focus-visible {
  outline: 2px solid var(--c-accent);
  outline-offset: 2px;
}
svg {
  display: block;
  width: 1.25rem;
  height: 1.25rem;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
  fill: none;
}
svg text {
  fill: currentColor;
  stroke: none;
  font-size: 0.6rem;
  font-weight: 600;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
.music-notice,
.music-error {
  padding: 0.75rem 1rem;
  border-radius: 1rem;
}
.music-error {
  margin-bottom: var(--space-2);
}
.music-notice button,
.music-error button {
  min-height: 2.75rem;
  padding-inline: var(--space-3);
  color: var(--c-accent);
}
.panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}
.panel-heading h2,
.queue-heading h3 {
  color: var(--c-accent);
  font-family: var(--font-heading);
  font-weight: 400;
}
.panel-heading h2 {
  font-size: var(--text-xl);
  letter-spacing: 0.04em;
}
.panel-heading button {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  min-height: 2.75rem;
  padding: 0.4rem 0.6rem;
  color: var(--c-accent);
  font-size: var(--text-sm);
}
.panel-heading button svg {
  width: 1rem;
  height: 1rem;
}
.now-playing {
  display: grid;
  grid-template-columns: 9.5rem minmax(0, 1fr) 8rem;
  align-items: center;
  gap: var(--space-5);
}
.now-cover,
.cover {
  position: relative;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  overflow: hidden;
  color: var(--c-accent);
  background:
    radial-gradient(circle at 68% 44%, var(--c-accent) 0 7%, transparent 7.5%),
    radial-gradient(
      circle at 68% 44%,
      color-mix(in srgb, var(--c-ink) 80%, var(--c-accent)) 0 34%,
      transparent 34.5%
    ),
    var(--c-surface-sunken);
}
.now-cover {
  width: 9.5rem;
  aspect-ratio: 1;
  border-radius: 0.8rem;
  box-shadow: 0 0.75rem 1.75rem color-mix(in srgb, var(--c-ink) 12%, transparent);
  font-family: var(--font-heading);
  font-size: 2rem;
}
.cover {
  width: 3.25rem;
  height: 3.25rem;
  border-radius: 0.65rem;
  font-size: 1.35rem;
}
.now-cover img,
.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.now-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
}
.now-kicker {
  margin-bottom: var(--space-2);
  color: var(--c-ink-faint);
  font-size: 0.6875rem;
  letter-spacing: 0.16em;
}
.now-copy strong {
  max-width: 100%;
  overflow: hidden;
  color: color-mix(in srgb, var(--c-ink) 76%, var(--c-accent));
  font-size: clamp(1.15rem, 2.5vw, 1.6rem);
  font-weight: 500;
  letter-spacing: -0.02em;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.now-artist {
  margin-top: var(--space-1);
  color: var(--c-ink-muted);
  font-size: var(--text-sm);
}
.playlist-select {
  width: auto;
  max-width: 12rem;
  margin-top: var(--space-3);
}
.seek-row {
  display: grid;
  grid-template-columns: 2.75rem minmax(0, 1fr) 2.75rem;
  align-items: center;
  gap: var(--space-3);
  margin-top: var(--space-4);
  color: var(--c-ink-muted);
  font-variant-numeric: tabular-nums;
}
.seek-row > span:last-child {
  text-align: right;
}
input[type='range'] {
  width: 100%;
  min-width: 0;
  height: 2rem;
  margin: 0;
  accent-color: var(--c-accent);
  cursor: pointer;
}
input:disabled {
  cursor: default;
  opacity: 0.45;
}
.playback-row {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 4.5rem;
  margin-top: var(--space-1);
}
.panel-transport {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.panel-transport button,
.transport button {
  display: grid;
  place-items: center;
  width: 2.75rem;
  min-height: 2.75rem;
  padding: 0;
  color: color-mix(in srgb, var(--c-ink) 66%, var(--c-accent));
}
.panel-transport button.active {
  color: var(--c-accent);
  background: var(--c-surface-sunken);
}
.panel-transport .panel-play,
.transport .play-button {
  color: var(--c-on-accent);
  background: var(--c-accent);
  box-shadow: 0 0.5rem 1.25rem color-mix(in srgb, var(--c-accent) 24%, transparent);
}
.panel-transport .panel-play {
  width: 4rem;
  min-height: 4rem;
  margin-inline: var(--space-1);
  border-radius: 50%;
}
.panel-play svg {
  width: 1.55rem;
  height: 1.55rem;
}
.volume-control {
  display: flex;
  align-items: center;
  align-self: end;
  justify-self: end;
  gap: var(--space-2);
  width: 8rem;
  padding-bottom: var(--space-1);
}
.volume-control label {
  color: var(--c-ink-muted);
}
.queue-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-top: var(--space-3);
  padding-top: var(--space-4);
  border-top: 1px solid var(--c-line);
}
.queue-heading h3 {
  font-size: var(--text-lg);
}
.queue-heading span {
  color: var(--c-ink-faint);
  font-variant-numeric: tabular-nums;
}
.song-list {
  max-height: 14rem;
  margin-top: var(--space-1);
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
  scrollbar-color: var(--c-line-strong) transparent;
}
.song-list button {
  display: grid;
  grid-template-columns: 2rem 1rem minmax(0, 1fr) auto;
  align-items: center;
  width: 100%;
  min-height: 3.5rem;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  text-align: left;
}
.song-list button:not([aria-current='true']) .song-text {
  grid-column: 2 / 4;
}
.song-list button[aria-current='true'] {
  color: var(--c-accent);
  background: color-mix(in srgb, var(--c-surface-sunken) 82%, var(--c-accent));
}
.song-number,
.song-duration {
  color: var(--c-ink-muted);
  font-variant-numeric: tabular-nums;
}
.song-duration {
  margin-left: var(--space-2);
}
.song-text,
.track-text {
  display: grid;
  min-width: 0;
}
.song-text > span,
.track-text > span,
small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.song-text > span {
  font-size: var(--text-sm);
}
small {
  color: var(--c-ink-muted);
  font-size: 0.75rem;
}
.equalizer {
  display: flex;
  align-items: end;
  justify-content: center;
  gap: 2px;
  height: 1rem;
}
.equalizer i {
  width: 2px;
  height: 45%;
  border-radius: 1px;
  background: var(--c-accent);
  animation: equalize 0.9s ease-in-out infinite alternate;
}
.equalizer i:nth-child(2) {
  height: 90%;
  animation-delay: -0.35s;
}
.equalizer i:nth-child(3) {
  height: 65%;
  animation-delay: -0.6s;
}
@keyframes equalize {
  to {
    height: 100%;
  }
}
.track-trigger {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  min-width: 0;
  gap: var(--space-3);
  padding: 0;
  text-align: left;
}
.track-trigger:hover {
  background: transparent;
}
.track-text > span {
  color: color-mix(in srgb, var(--c-ink) 78%, var(--c-accent));
  font-size: var(--text-sm);
}
.expand-mark {
  margin-left: auto;
  padding: var(--space-2);
  color: var(--c-ink-muted);
}
.expand-mark svg {
  width: 1rem;
  height: 1rem;
}
.bar-progress {
  position: relative;
  z-index: 1;
  height: 2px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--c-line);
}
.bar-progress span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--c-accent);
}
.transport {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: var(--space-1);
}
.transport .play-button {
  width: 3.25rem;
  min-height: 3.25rem;
  border-radius: 1rem;
}
.music-expand-enter-active,
.music-expand-leave-active {
  transition:
    opacity var(--duration-mid) var(--ease-out),
    transform var(--duration-mid) var(--ease-out);
  transform-origin: bottom center;
}
.music-expand-enter-from,
.music-expand-leave-to {
  opacity: 0;
  transform: translateY(0.75rem) scale(0.985);
}
@media (max-width: 46rem) {
  .music-player {
    bottom: max(0.625rem, env(safe-area-inset-bottom));
    width: calc(100vw - 1.5rem);
  }
  .music-panel {
    max-height: calc(100dvh - 6.5rem);
    padding: var(--space-4);
    border-radius: 1.25rem;
  }
  .now-playing {
    grid-template-columns: 6rem minmax(0, 1fr);
    gap: var(--space-4);
  }
  .now-cover {
    width: 6rem;
  }
  .volume-control {
    display: none;
  }
  .music-bar {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: var(--space-2);
  }
  .bar-progress {
    display: none;
  }
}
@media (max-width: 30rem) {
  .music-panel {
    padding: var(--space-4) var(--space-3) var(--space-3);
  }
  .panel-heading {
    margin-bottom: var(--space-3);
  }
  .panel-heading h2 {
    font-size: var(--text-lg);
  }
  .now-playing {
    grid-template-columns: 4.75rem minmax(0, 1fr);
    gap: var(--space-3);
  }
  .now-cover {
    width: 4.75rem;
  }
  .now-kicker {
    display: none;
  }
  .now-copy strong {
    font-size: 1rem;
  }
  .seek-row {
    gap: var(--space-2);
  }
  .panel-transport {
    gap: 0;
  }
  .queue-heading {
    margin-top: var(--space-2);
  }
  .song-list {
    max-height: 11rem;
  }
  .song-list button {
    padding-inline: var(--space-2);
  }
  .cover {
    width: 2.75rem;
    height: 2.75rem;
  }
  .transport button {
    width: 2.5rem;
  }
  .transport .play-button {
    width: 2.75rem;
    min-height: 2.75rem;
    border-radius: 0.85rem;
  }
}
@supports not ((backdrop-filter: blur(1px))) {
  .music-bar,
  .music-panel,
  .music-notice,
  .music-error {
    background: var(--music-glass-fallback);
  }
}
@media (prefers-reduced-transparency: reduce), (forced-colors: active) {
  .music-bar,
  .music-panel,
  .music-notice,
  .music-error {
    background: var(--music-glass-fallback);
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }
}
@media (prefers-reduced-motion: reduce) {
  button,
  .music-bar,
  .music-expand-enter-active,
  .music-expand-leave-active {
    transition: none;
  }
  button:active {
    transform: none;
  }
  .equalizer i {
    height: 65%;
    animation: none;
  }
}
</style>
