<!-- 咨询前的资料预填卡片, 昵称与邮箱都是选填 -->
<template>
  <div class="chat-prechat">
    <div class="chat-prechat-title">{{ title }}</div>
    <div class="chat-prechat-desc">{{ t('chat.prechat.desc') }}</div>
    <ElForm labelPosition="top" @submit.prevent="">
      <ElFormItem :label="t('chat.prechat.nickname')">
        <ElInput
          v-model.trim="nickname"
          maxlength="32"
          :placeholder="t('chat.prechat.nicknamePlaceholder')"
          clearable
        />
      </ElFormItem>
      <ElFormItem :label="t('chat.prechat.email')" :error="emailError">
        <ElInput
          v-model.trim="email"
          maxlength="128"
          :placeholder="t('chat.prechat.emailPlaceholder')"
          clearable
        />
      </ElFormItem>
    </ElForm>
    <div class="chat-prechat-footer">
      <ElButton v-if="showSkip" text @click="emit('skip')">
        {{ t('chat.prechat.skip') }}
      </ElButton>
      <ElButton type="primary" :loading="loading" @click="submit">
        {{ t('chat.prechat.start') }}
      </ElButton>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { computed, ref, watch } from 'vue';
  import type { PropType } from 'vue';
  import { useI18n } from 'vue-i18n';
  import type { VisitorProfile } from '@/api/types';

  defineOptions({ name: 'ChatPrechatForm' });

  const props = defineProps({
    /** 已保存的访客资料 */
    profile: Object as PropType<VisitorProfile | null>,
    /** 提交中 */
    loading: Boolean,
    /** 是否展示跳过按钮, 修改资料时不展示 */
    showSkip: Boolean
  });

  const emit = defineEmits({
    /** 提交资料 */
    submit: (_profile: VisitorProfile) => true,
    /** 跳过预填 */
    skip: () => true
  });

  const { t } = useI18n();

  /** 昵称 */
  const nickname = ref('');

  /** 邮箱 */
  const email = ref('');

  /** 邮箱格式错误提示 */
  const emailError = ref('');

  /** 卡片标题, 修改资料时用另一套文案 */
  const title = computed(() =>
    props.showSkip ? t('chat.prechat.title') : t('chat.prechat.editTitle')
  );

  /** 同步外部资料到输入框 */
  watch(
    () => props.profile,
    (value) => {
      nickname.value = value?.nickname ?? '';
      email.value = value?.email ?? '';
    },
    { immediate: true }
  );

  /** 邮箱格式校验, 与后端保持一致的宽松规则 */
  const isValidEmail = (value: string) =>
    /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);

  /** 提交资料 */
  const submit = () => {
    const value = email.value.trim();
    if (value && !isValidEmail(value)) {
      emailError.value = t('chat.prechat.emailInvalid');
      return;
    }
    emailError.value = '';
    emit('submit', { nickname: nickname.value.trim(), email: value });
  };
</script>

<style lang="scss" scoped>
  .chat-prechat {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    justify-content: center;
    padding: 24px 20px;
    background: var(--el-bg-color);
    gap: 12px;
  }

  .chat-prechat-title {
    font-size: 16px;
    font-weight: 600;
  }

  .chat-prechat-desc {
    font-size: 12px;
    line-height: 1.6;
    color: var(--el-text-color-secondary);
  }

  .chat-prechat-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
