<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
  eyebrow?: string
}>()
</script>

<template>
  <section class="page-frame">
    <div class="page-frame__glow page-frame__glow--primary"></div>
    <div class="page-frame__glow page-frame__glow--secondary"></div>

    <header class="page-frame__header">
      <div class="page-frame__heading">
        <span v-if="eyebrow" class="page-frame__eyebrow">{{ eyebrow }}</span>
        <h1>{{ title }}</h1>
        <p v-if="subtitle">{{ subtitle }}</p>
      </div>
      <div v-if="$slots.actions" class="page-frame__actions">
        <slot name="actions" />
      </div>
    </header>

    <div v-if="$slots.metrics" class="page-frame__metrics">
      <slot name="metrics" />
    </div>

    <div class="page-frame__content">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.page-frame {
  position: relative;
  padding: 32px;
  overflow: hidden;
}

.page-frame__glow {
  position: absolute;
  border-radius: 999px;
  filter: blur(80px);
  opacity: 0.18;
  pointer-events: none;
}

.page-frame__glow--primary {
  width: 320px;
  height: 320px;
  top: -120px;
  right: -80px;
  background: var(--color-accent);
}

.page-frame__glow--secondary {
  width: 280px;
  height: 280px;
  bottom: -120px;
  left: -40px;
  background: var(--color-warning);
}

.page-frame__header,
.page-frame__metrics,
.page-frame__content {
  position: relative;
  z-index: 1;
}

.page-frame__header {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-frame__heading {
  max-width: 720px;
}

.page-frame__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  margin-bottom: 12px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--color-accent) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-accent) 18%, transparent);
  color: var(--color-accent);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

.page-frame__heading h1 {
  margin: 0;
  font-size: clamp(28px, 4vw, 40px);
  font-weight: 700;
  letter-spacing: -0.03em;
  color: var(--color-text-primary);
}

.page-frame__heading p {
  margin: 10px 0 0;
  max-width: 640px;
  color: var(--color-text-secondary);
  font-size: 15px;
  line-height: 1.7;
}

.page-frame__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.page-frame__metrics {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  margin-bottom: 24px;
}

.page-frame__content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

@media (max-width: 768px) {
  .page-frame {
    padding: 20px;
  }

  .page-frame__header {
    flex-direction: column;
  }

  .page-frame__actions {
    width: 100%;
    justify-content: stretch;
  }

  .page-frame__actions :deep(*) {
    flex: 1;
  }
}

@media (max-width: 520px) {
  .page-frame {
    padding: 16px;
  }

  .page-frame__metrics {
    grid-template-columns: 1fr;
  }
}
</style>
