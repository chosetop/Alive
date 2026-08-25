<script setup lang="ts">
import type { NuxtError } from '#app'

/**
 * Root error page. Nuxt renders this for unmatched routes (404) and for uncaught
 * errors during rendering, replacing the whole app — so it sits at the project
 * root rather than under `pages/`, and it mounts its own layout.
 */
const props = defineProps<{ error: NuxtError }>()

const isNotFound = computed(() => props.error.statusCode === 404)

/**
 * The message set at the throw site, when there is one. Those are written for
 * readers ('没有这一篇'), which is more use than a generic string.
 *
 * Reads `message` rather than `statusMessage`: h3 treats statusMessage as the
 * HTTP reason phrase and sanitises it, so non-ASCII text does not survive there.
 */
const message = computed(() => {
  if (props.error.message) return props.error.message
  return isNotFound.value ? '没有这一页' : '出了点问题'
})

const detail = computed(() =>
  isNotFound.value
    ? '这个地址下没有内容。可能是链接错了，或者它曾经存在过。'
    : '页面没能正常读出来。稍后再试一次，或者回到首页。',
)

useHead({
  title: computed(() => `${message.value} · Alive`),
  /** An error page has nothing worth indexing, whatever its status code. */
  meta: [{ name: 'robots', content: 'noindex' }],
})

/** Clears the error state and navigates, rather than reloading the document. */
function goHome(): void {
  clearError({ redirect: '/' })
}
</script>

<template>
  <NuxtLayout>
    <section class="err">
      <p class="code" aria-hidden="true">{{ error.statusCode }}</p>
      <h1 class="title">{{ message }}</h1>
      <p class="detail">{{ detail }}</p>
      <button class="back" type="button" @click="goHome">← 回到首页</button>
    </section>
  </NuxtLayout>
</template>

<style scoped>
.err {
  max-width: var(--measure);
  padding-block: var(--space-9);
}

/*
 * The number is set large and faint: it identifies the failure for anyone who
 * knows what it means, without becoming the message for anyone who does not.
 *
 * `--c-ink-faint` (3.2:1), not `--c-line-strong` (1.8:1). The border token is
 * tuned for hairlines and is effectively invisible as type. This is exempt from
 * the 4.5:1 minimum either way, being aria-hidden and restated by the title
 * below, but "faint" has to mean legible.
 */
.code {
  color: var(--c-ink-faint);
  font-size: var(--text-3xl);
  font-variant-numeric: tabular-nums;
  line-height: var(--leading-none);
  letter-spacing: var(--tracking-display);
}

.title {
  margin-top: var(--space-4);
  font-size: var(--text-xl);
}

.detail {
  margin-top: var(--space-3);
  color: var(--c-ink-muted);
  font-size: var(--text-sm);
}

.back {
  margin-top: var(--space-6);
  padding: 0;
  border: 0;
  background: none;
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-out);
}

.back:hover {
  color: var(--c-accent);
}
</style>
