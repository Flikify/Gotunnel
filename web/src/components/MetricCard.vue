<script setup lang="ts">
const props = defineProps<{
  label: string
  value: string | number
  hint?: string
  tone?: 'default' | 'success' | 'warning' | 'info'
}>()

const toneClass = props.tone || 'default'
</script>

<template>
  <article class="metric-card" :class="`metric-card--${toneClass}`">
    <span class="metric-card__label">{{ label }}</span>
    <strong class="metric-card__value">{{ value }}</strong>
    <span v-if="hint" class="metric-card__hint">{{ hint }}</span>
  </article>
</template>

<style scoped>
.metric-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px;
  min-height: 128px;
  border-radius: 20px;
  background: var(--glass-bg-card);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-card);
}

.metric-card__label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.metric-card__value {
  font-size: clamp(26px, 4vw, 34px);
  line-height: 1;
  color: var(--color-text-primary);
  letter-spacing: -0.04em;
}

.metric-card__hint {
  margin-top: auto;
  font-size: 13px;
  color: var(--color-text-muted);
}

.metric-card--success {
  border-color: rgba(16, 185, 129, 0.24);
}

.metric-card--warning {
  border-color: rgba(245, 158, 11, 0.24);
}

.metric-card--info {
  border-color: rgba(6, 182, 212, 0.24);
}
</style>
