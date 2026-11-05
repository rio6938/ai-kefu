<!-- 气泡加载中, 提示访客客服已收到消息 -->
<template>
  <div class="chat-loading-bubble" role="status" aria-live="polite">
    <span class="chat-loading-dots">
      <i></i>
      <i></i>
      <i></i>
    </span>
    <span v-if="text" class="chat-loading-text">{{ text }}</span>
  </div>
</template>

<script lang="ts" setup>
  defineOptions({ name: 'LoadingBubble' });

  defineProps({
    /** 加载提示文案, 为空时只展示动画 */
    text: String
  });
</script>

<style lang="scss" scoped>
  .chat-loading-bubble {
    display: inline-flex;
    align-items: center;
    min-height: 20px;
    gap: 8px;
  }

  .chat-loading-dots {
    display: inline-flex;
    align-items: center;
    gap: 3px;

    i {
      display: block;
      width: 6px;
      height: 6px;
      background: var(--el-text-color-secondary);
      border-radius: 50%;
      animation: chat-loading-bounce 1.2s infinite ease-in-out;

      &:nth-child(2) {
        animation-delay: 0.15s;
      }

      &:nth-child(3) {
        animation-delay: 0.3s;
      }
    }
  }

  .chat-loading-text {
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }

  @keyframes chat-loading-bounce {
    0%,
    60%,
    100% {
      opacity: 0.35;
      transform: translateY(0);
    }
    30% {
      opacity: 1;
      transform: translateY(-3px);
    }
  }
</style>
