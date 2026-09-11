<template>
  <div class="world-browse">
    <aside
      class="world-browse__rail"
      :class="{ 'world-browse__rail--empty': !$slots.header && !$slots.navigation }"
    >
      <header v-if="$slots.header" class="world-browse__head">
        <slot name="header" />
      </header>

      <div class="world-browse__margin">
        <slot name="navigation" />
      </div>
    </aside>

    <div class="world-browse__body">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.world-browse {
  position: relative;
  left: 50%;
  display: grid;
  grid-template-columns: clamp(11rem, 15vw, 16rem) minmax(0, 1fr);
  column-gap: clamp(var(--space-5), 3vw, var(--space-7));
  row-gap: var(--space-7);
  width: min(84vw, 88rem);
  transform: translateX(-50%);
}

.world-browse__rail {
  position: sticky;
  top: calc(var(--alive-masthead-height, 0px) + var(--space-2));
  align-self: start;
  min-width: 0;
  max-height: calc(100dvh - var(--alive-masthead-height, 0px) - var(--space-4));
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--c-line-strong) transparent;
}

.world-browse__head {
  min-width: 0;
  padding-top: var(--space-2);
}

.world-browse__body {
  min-width: 0;
}

.world-browse__margin {
  margin-top: var(--space-6);
}

.world-browse__rail :deep(.category-index) {
  position: static;
  top: auto;
}

.world-browse__rail--empty + .world-browse__body {
  grid-column: 1 / -1;
}

@media (max-width: 64rem) {
  .world-browse {
    position: static;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-5);
    width: 100%;
    transform: none;
  }

  .world-browse__rail {
    position: static;
    max-height: none;
    overflow: visible;
  }

  .world-browse__head {
    padding-top: 0;
  }

  .world-browse__margin {
    margin-top: 0;
  }

  .world-browse__rail--empty + .world-browse__body {
    grid-column: auto;
  }
}
</style>
