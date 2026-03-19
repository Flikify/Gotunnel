<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
  eyebrow?: string
}>()
</script>

<template>
  <section class="page-shell">
    <div class="page-shell__glow page-shell__glow--primary"></div>
    <div class="page-shell__glow page-shell__glow--secondary"></div>

    <header class="page-shell__header">
      <div class="page-shell__heading">
        <span v-if="eyebrow" class="page-shell__eyebrow">{{ eyebrow }}</span>
        <h1>{{ title }}</h1>
        <p v-if="subtitle">{{ subtitle }}</p>
      </div>
      <div v-if="$slots.actions" class="page-shell__actions">
        <slot name="actions" />
      </div>
    </header>

    <div v-if="$slots.metrics" class="page-shell__metrics">
      <slot name="metrics" />
    </div>

    <div class="page-shell__content">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.page-shell {
  position: relative;
  padding: 32px;
  overflow: hidden;
}

.page-shell__glow {
  position: absolute;
  border-radius: 999px;
  filter: blur(80px);
  opacity: 0.18;
  pointer-events: none;
}

.page-shell__glow--primary {
  width: 320px;
  height: 320px;
  top: -120px;
  right: -80px;
  background: var(--color-accent);
}

.page-shell__glow--secondary {
  width: 280px;
  height: 280px;
  bottom: -120px;
  left: -40px;
  background: #8b5cf6;
}

.page-shell__header,
.page-shell__metrics,
.page-shell__content {
  position: relative;
  z-index: 1;
}

.page-shell__header {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-shell__heading {
  max-width: 720px;
}

.page-shell__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  margin-bottom: 12px;
  border-radius: 999px;
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.18);
  color: var(--color-accent);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

.page-shell__heading h1 {
  margin: 0;
  font-size: clamp(28px, 4vw, 40px);
  font-weight: 700;
  letter-spacing: -0.03em;
  color: var(--color-text-primary);
}

.page-shell__heading p {
  margin: 10px 0 0;
  max-width: 640px;
  color: var(--color-text-secondary);
  font-size: 15px;
  line-height: 1.7;
}

.page-shell__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.page-shell__metrics {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  margin-bottom: 24px;
}

.page-shell__content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

@media (max-width: 768px) {
  .page-shell {
    padding: 20px;
  }

  .page-shell__header {
    flex-direction: column;
  }

  .page-shell__actions {
    width: 100%;
    justify-content: stretch;
  }

  .page-shell__actions :deep(*) {
    flex: 1;
  }
}
</style>
