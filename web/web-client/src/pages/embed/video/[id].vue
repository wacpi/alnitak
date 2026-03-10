<template>
  <div></div>
</template>

<script setup lang="ts">
const route = useRoute();

const id = route.params.id;
const p = route.query.p;

if (id) {
  navigateTo({
    path: '/embed/watch',
    query: { v: String(id), ...(p && { p: Array.isArray(p) ? p[0] : p }) }
  });
} else {
  navigateTo('/404');
}
</script>

<style scoped>
/* redirect page, no styles */
</style>
