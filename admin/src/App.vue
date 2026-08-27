<script setup lang="ts">
import { MilkdownProvider } from '@milkdown/vue'

/**
 * Root component. Deliberately nothing but the outlet: the authenticated
 * chrome belongs to AdminLayout, which the router mounts for the routes that
 * need it, so /login renders without it.
 */
</script>

<template>
  <MilkdownProvider>
    <RouterView />
  </MilkdownProvider>
</template>
