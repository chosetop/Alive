<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { isApiClientError, toUserMessage } from '../api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const isSubmitting = ref(false)
const errorMessage = ref('')

async function handleSubmit(): Promise<void> {
  // The button is disabled while submitting, but Enter in a text field can
  // still fire submit, so the guard is here rather than only in the markup.
  if (isSubmitting.value) return

  isSubmitting.value = true
  errorMessage.value = ''

  try {
    await auth.login({ username: username.value, password: password.value })

    // Return them to whatever they were trying to reach. Only same-site paths
    // are honoured: `redirect` comes from the URL, and following an absolute
    // URL from it would make this form an open redirect.
    const redirect = route.query.redirect
    const target = typeof redirect === 'string' && redirect.startsWith('/') ? redirect : '/dashboard'
    await router.replace(target)
  } catch (error) {
    // toUserMessage maps the code, so INVALID_CREDENTIALS reads as
    // "用户名或密码错误" and a 500 never shows the server's own wording.
    errorMessage.value = toUserMessage(error)
    // Clear the password but keep the username: a mistyped password is the
    // likely cause, and retyping both is busywork.
    password.value = ''
    if (isApiClientError(error) && error.requestId) {
      // Useful when someone reports "login is broken"; harmless otherwise.
      console.error(`login failed (request_id: ${error.requestId})`)
    }
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <main class="page">
    <div class="panel">
      <h1 class="wordmark">Alive</h1>

      <!--
        The store records an initialization failure separately from "not logged
        in". Saying so here explains why signing in is about to fail, instead of
        letting them conclude they forgot their password.
      -->
      <p v-if="auth.initializationError" class="notice" role="status">
        无法连接到服务器，登录可能不可用。
      </p>

      <form class="form" novalidate @submit.prevent="handleSubmit">
        <div class="field">
          <label class="label" for="username">用户名</label>
          <input
            id="username"
            v-model="username"
            class="input"
            type="text"
            name="username"
            autocomplete="username"
            required
            :disabled="isSubmitting"
          />
        </div>

        <div class="field">
          <label class="label" for="password">密码</label>
          <input
            id="password"
            v-model="password"
            class="input"
            type="password"
            name="password"
            autocomplete="current-password"
            required
            :disabled="isSubmitting"
          />
        </div>

        <!--
          role="alert" so a screen reader announces the failure. Without it the
          message appears silently and a non-sighted user is left with a form
          that simply did not proceed.
        -->
        <p v-if="errorMessage" class="error" role="alert">{{ errorMessage }}</p>

        <button class="submit" type="submit" :disabled="isSubmitting">
          {{ isSubmitting ? '登录中…' : '登录' }}
        </button>
      </form>
    </div>
  </main>
</template>

<style scoped>
.page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: var(--space-6) var(--space-4);
}

.panel {
  width: 100%;
  /* Narrow on purpose. A login form stretched to a container's width reads as
     unfinished, and the fields become harder to scan. */
  max-width: 20rem;
}

.wordmark {
  margin-bottom: var(--space-6);
  font-size: 1.5rem;
  letter-spacing: -0.02em;
}

.notice {
  margin-bottom: var(--space-4);
  padding: var(--space-2) var(--space-3);
  border-left: 2px solid var(--c-line-strong);
  background: var(--c-surface-sunken);
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.form {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.label {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.input {
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  background: var(--c-surface);
  color: var(--c-ink);
  transition: border-color 0.12s ease;
}

.input:hover:not(:disabled) {
  border-color: var(--c-ink-faint);
}

.input:disabled {
  background: var(--c-surface-sunken);
  color: var(--c-ink-faint);
}

.error {
  color: var(--c-danger);
  font-size: 0.8125rem;
}

.submit {
  padding: 0.5rem 0.75rem;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: var(--c-accent);
  color: #fff;
  cursor: pointer;
  transition: background-color 0.12s ease;
}

.submit:hover:not(:disabled) {
  background: var(--c-accent-hover);
}

.submit:disabled {
  background: var(--c-ink-faint);
  cursor: not-allowed;
}
</style>
