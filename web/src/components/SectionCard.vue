<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <section class="section-card glass-card">
    <header class="section-card__header">
      <div>
        <h2>{{ title }}</h2>
        <p v-if="description">{{ description }}</p>
      </div>
      <div v-if="$slots.header" class="section-card__extra">
        <slot name="header" />
      </div>
    </header>

    <div class="section-card__body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.section-card {
  padding: 22px;
}

.section-card__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 18px;
}

.section-card__header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--color-text-primary);
}

.section-card__header p {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-secondary);
}

.section-card__extra {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.section-card__body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

@media (max-width: 768px) {
  .section-card {
    padding: 18px;
  }

  .section-card__header {
    flex-direction: column;
  }

  .section-card__extra {
    width: 100%;
  }
}
</style>
