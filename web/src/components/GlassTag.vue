<script setup lang="ts">
defineProps<{
  type?: 'default' | 'success' | 'warning' | 'error' | 'info'
  size?: 'small' | 'medium'
  round?: boolean
}>()
</script>

<template>
  <span class="glass-tag" :class="[type || 'default', size || 'small', { round }]">
    <slot />
  </span>
</template>

<style scoped>
.glass-tag {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.15);
}

.glass-tag.round {
  border-radius: 20px;
}

.glass-tag.medium {
  padding: 4px 12px;
  font-size: 13px;
}

.glass-tag.success {
  background: rgba(52, 211, 153, 0.15);
  border-color: rgba(52, 211, 153, 0.3);
  color: #34d399;
}

.glass-tag.warning {
  background: rgba(251, 191, 36, 0.15);
  border-color: rgba(251, 191, 36, 0.3);
  color: #fbbf24;
}

.glass-tag.error {
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.3);
  color: #f87171;
}

.glass-tag.info {
  background: rgba(96, 165, 250, 0.15);
  border-color: rgba(96, 165, 250, 0.3);
  color: #60a5fa;
}
</style>
