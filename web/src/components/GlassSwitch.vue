<script setup lang="ts">
const props = defineProps<{
  modelValue?: boolean
  size?: 'small' | 'medium'
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const toggle = () => {
  if (!props.disabled) {
    emit('update:modelValue', !props.modelValue)
  }
}
</script>

<template>
  <button
    class="glass-switch"
    :class="[size || 'small', { active: modelValue, disabled }]"
    @click="toggle"
    :disabled="disabled"
  >
    <span class="switch-thumb"></span>
  </button>
</template>

<style scoped>
.glass-switch {
  position: relative;
  width: 36px;
  height: 20px;
  background: rgba(255, 255, 255, 0.15);
  border: none;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  padding: 0;
}

.glass-switch.small {
  width: 32px;
  height: 18px;
}

.glass-switch:hover:not(.disabled) {
  background: rgba(255, 255, 255, 0.25);
}

.glass-switch.active {
  background: var(--gradient-accent);
}

.glass-switch.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.switch-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 14px;
  height: 14px;
  background: white;
  border-radius: 50%;
  transition: transform 0.2s;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.glass-switch.small .switch-thumb {
  width: 12px;
  height: 12px;
}

.glass-switch.active .switch-thumb {
  transform: translateX(16px);
}

.glass-switch.small.active .switch-thumb {
  transform: translateX(14px);
}
</style>
