<!-- 输入框上方的快捷问题胶囊, 点击直接发送 -->
<template>
  <div v-if="items.length" class="chat-quick-replies">
    <ElTag
      v-for="item in items"
      :key="item.quickReplyId"
      size="small"
      effect="plain"
      class="chat-quick-replies-item"
      :disableTransitions="true"
      @click="emit('select', item)"
    >
      {{ item.title }}
    </ElTag>
  </div>
</template>

<script lang="ts" setup>
  import type { PropType } from 'vue';
  import type { QuickReplyItem } from '@/api/types';

  defineOptions({ name: 'ChatQuickReplies' });

  defineProps({
    /** 快捷问题列表, 为空时整块不渲染 */
    items: {
      type: Array as PropType<QuickReplyItem[]>,
      default: () => []
    }
  });

  const emit = defineEmits({
    /** 选中一条快捷问题 */
    select: (_item: QuickReplyItem) => true
  });
</script>

<style lang="scss" scoped>
  .chat-quick-replies {
    display: flex;
    padding: 8px 0 4px;
    overflow-x: auto;
    gap: 8px;
    scrollbar-width: none;

    &::-webkit-scrollbar {
      display: none;
    }
  }

  .chat-quick-replies-item {
    flex: none;
    cursor: pointer;
  }
</style>
