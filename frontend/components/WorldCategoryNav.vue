<script setup lang="ts">
import { computed } from 'vue'

import { resolvePublicWorld } from '~/content-worlds/registry'
import type { Category, WorldKey } from '~/types'

const props = defineProps<{
  world: WorldKey
  categories: Category[]
  activeSlug?: string
}>()

const definition = computed(() => resolvePublicWorld(props.world))
const visibleCategories = computed(() => props.categories)
</script>

<template>
  <nav v-if="definition && visibleCategories.length > 0" class="category-index" :aria-label="`${definition.label}分类`">
    <div class="links">
      <NuxtLink
        v-for="category in visibleCategories"
        :key="category.slug"
        class="link"
        :class="{ 'link--active': category.slug === activeSlug }"
        :aria-current="category.slug === activeSlug ? 'page' : undefined"
        :to="definition.categoryPath(category.slug)"
      >
        {{ category.name }}
        <span class="count">{{ category.entry_count }}</span>
      </NuxtLink>
    </div>
  </nav>
</template>

<style scoped>
.category-index {
  position: sticky;
  top: calc(var(--alive-masthead-height, 0px) + var(--space-2));
  padding: var(--space-2) var(--space-6) var(--space-2) 0;
  border-right: 1px solid var(--c-line);
}

.links {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.links::before {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 1px;
  background: color-mix(in srgb, var(--c-accent) 62%, var(--c-line));
  content: '';
}

.link {
  position: relative;
  display: inline-flex;
  align-items: baseline;
  gap: var(--space-2);
  min-height: 2.8rem;
  padding: 0.62rem var(--space-3) 0.62rem var(--space-6);
  border-radius: 0 1rem 1rem 0;
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  line-height: 1.35;
  text-decoration: none;
  transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out);
}

.link:hover,
.link:focus-visible {
  background: color-mix(in srgb, var(--c-surface) 92%, var(--c-accent) 8%);
  color: var(--c-accent);
  transform: translateX(0.15rem);
}

.link--active {
  background: color-mix(in srgb, var(--c-surface) 82%, var(--c-accent) 18%);
  color: var(--c-accent);
}

.link::before {
  position: absolute;
  top: 50%;
  left: -0.17rem;
  width: 0.38rem;
  height: 0.38rem;
  border-radius: 50%;
  background: var(--c-accent);
  content: '';
  transform: translateY(-50%);
}

.link--active::before {
  left: -0.23rem;
  width: 0.48rem;
  height: 1.8rem;
  border-radius: 999px;
}

.count {
  margin-left: auto;
  color: var(--c-ink-faint);
  font-size: 0.625rem;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.04em;
}

.link--active .count {
  display: inline-grid;
  min-width: 1.75rem;
  height: 1.75rem;
  padding-inline: 0.35rem;
  place-items: center;
  border-radius: 999px;
  background: var(--c-accent);
  color: var(--c-surface);
  font-size: 0.72rem;
}

@media (max-width: 64rem) {
  .category-index {
    position: static;
    padding-block: var(--space-2);
    border-block: 1px solid var(--c-line);
    border-right: 0;
  }

  .links {
    flex-direction: row;
    flex-wrap: nowrap;
    margin-inline: calc(var(--space-5) * -1);
    padding-inline: var(--space-5);
    overflow-x: auto;
    scrollbar-width: none;
  }

  .links::-webkit-scrollbar {
    display: none;
  }

  .link {
    flex: 0 0 auto;
    min-height: 2.75rem;
    margin-left: 0;
    padding-inline: var(--space-3);
    border-left: 0;
    border-radius: 0.7rem;
    align-items: center;
  }

  .links::before,
  .link::before {
    display: none;
  }

  .link:hover,
  .link:focus-visible {
    transform: translateY(-1px);
  }

  .link--active::after {
    position: absolute;
    left: 50%;
    bottom: -0.55rem;
    width: 1rem;
    height: 0.15rem;
    border-radius: 0.15rem 0.15rem 0 0;
    background: var(--c-accent);
    content: '';
    transform: translateX(-50%);
  }

  .link--active .count {
    display: inline;
    min-width: 0;
    height: auto;
    padding: 0;
    background: transparent;
    color: var(--c-ink-faint);
    font-size: 0.625rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .link {
    transition: none;
  }
}
</style>
