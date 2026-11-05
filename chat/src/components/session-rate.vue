<!-- 会话结束后的满意度评价卡片 -->
<template>
  <div class="chat-rate">
    <div class="chat-rate-title">{{ t('chat.rate.title') }}</div>
    <ElRate
      v-model="score"
      :max="5"
      :disabled="submitting"
      class="chat-rate-stars"
    />
    <div class="chat-rate-tags">
      <ElCheckTag
        v-for="item in tagOptions"
        :key="item.value"
        :checked="tags.includes(item.value)"
        :disabled="submitting"
        @change="(checked: boolean) => toggleTag(item.value, checked)"
      >
        {{ item.label }}
      </ElCheckTag>
    </div>
    <ElInput
      type="textarea"
      resize="none"
      :rows="2"
      :maxlength="200"
      showWordLimit
      :disabled="submitting"
      :placeholder="t('chat.rate.remarkPlaceholder')"
      v-model="remark"
    />
    <div class="chat-rate-actions">
      <ElButton text :disabled="submitting" @click="emit('skip')">
        {{ t('chat.rate.skip') }}
      </ElButton>
      <ElButton type="primary" :loading="submitting" @click="submit">
        {{ t('chat.rate.submit') }}
      </ElButton>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';

  defineOptions({ name: 'SessionRate' });

  const props = defineProps({
    /** 提交中 */
    submitting: Boolean
  });

  const emit = defineEmits({
    /** 提交评价 */
    submit: (_value: { score: number; tags: string[]; remark: string }) => true,
    /** 暂不评价 */
    skip: () => true
  });

  const { t } = useI18n();

  /** 评分, 0 表示未选 */
  const score = ref(5);

  /** 已选标签 */
  const tags = ref<string[]>([]);

  /** 评价备注 */
  const remark = ref('');

  /** 可选标签, 与服务端字典 kefu_rate_tag 的默认数据保持一致 */
  const tagOptions = computed(() => [
    { value: t('chat.rate.tags.speed'), label: t('chat.rate.tags.speed') },
    { value: t('chat.rate.tags.professional'), label: t('chat.rate.tags.professional') },
    { value: t('chat.rate.tags.attitude'), label: t('chat.rate.tags.attitude') },
    { value: t('chat.rate.tags.solved'), label: t('chat.rate.tags.solved') }
  ]);

  /** 切换标签选中状态 */
  const toggleTag = (value: string, checked: boolean) => {
    if (props.submitting) {
      return;
    }
    tags.value = checked
      ? [...tags.value, value]
      : tags.value.filter((item) => item !== value);
  };

  /** 提交评价, 星级必填 */
  const submit = () => {
    if (props.submitting || score.value < 1) {
      return;
    }
    emit('submit', {
      score: score.value,
      tags: tags.value,
      remark: remark.value.trim()
    });
  };
</script>

<style lang="scss" scoped>
  .chat-rate {
    padding: 12px;
    background: var(--el-bg-color);
    border-top: 1px solid var(--el-border-color-lighter);
  }

  .chat-rate-title {
    margin-bottom: 8px;
    font-weight: 600;
  }

  .chat-rate-stars {
    margin-bottom: 8px;
  }

  .chat-rate-tags {
    display: flex;
    flex-wrap: wrap;
    margin-bottom: 8px;
    gap: 6px;
  }

  .chat-rate-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    margin-top: 8px;
    gap: 8px;
  }
</style>
