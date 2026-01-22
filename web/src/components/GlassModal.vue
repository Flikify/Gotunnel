<script setup lang="ts">
import { CloseOutline } from '@vicons/ionicons5'

defineProps<{
  show: boolean
  title: string
  width?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="modal-overlay" @click.self="emit('close')">
      <div class="modal-container" :style="{ maxWidth: width || '500px' }">
        <div class="modal-header">
          <h3>{{ title }}</h3>
          <button class="close-btn" @click="emit('close')">
            <CloseOutline />
          </button>
        </div>
        <div class="modal-body">
          <slot />
        </div>
        <div class="modal-footer">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 24px;
}

.modal-container {
  width: 100%;
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  border: 1px solid var(--color-border);
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border-light);
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.close-btn {
  background: var(--color-bg-elevated);
  border: none;
  border-radius: 6px;
  padding: 6px;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.15s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-btn:hover {
  background: rgba(244, 33, 46, 0.15);
  color: var(--color-error);
}

.close-btn svg {
  width: 18px;
  height: 18px;
}

.modal-body {
  padding: 20px;
}

.modal-footer {
  padding: 16px 20px;
  border-top: 1px solid var(--color-border-light);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.modal-footer:empty {
  display: none;
}
</style>
