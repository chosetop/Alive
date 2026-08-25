<script setup lang="ts">
/**
 * Root component. Deliberately nothing but the outlet: the authenticated
 * chrome belongs to AdminLayout, which the router mounts for the routes that
 * need it, so /login renders without it.
 */
</script>

<template>
  <RouterView />
</template>
