<!-- 客服离线时的留言卡片 -->
<template>
  <div class="chat-leave">
    <template v-if="done">
      <div class="chat-leave-done">
        <span class="chat-leave-done-icon">✓</span>
        <span>{{ t('chat.leave.submitted') }}</span>
      </div>
    </template>
    <template v-else>
      <div class="chat-leave-title">{{ t('chat.leave.title') }}</div>
      <div class="chat-leave-desc">{{ t('chat.leave.desc') }}</div>
      <div class="chat-leave-form">
        <ElInput
          v-model="form.name"
          :disabled="submitting"
          :placeholder="t('chat.leave.namePlaceholder')"
          :maxlength="64"
        />
        <ElInput
          v-model="form.contact"
          :disabled="submitting"
          :placeholder="t('chat.leave.contactPlaceholder')"
          :maxlength="128"
        />
        <ElInput
          v-model="form.content"
          type="textarea"
          resize="none"
          :rows="3"
          :maxlength="500"
          showWordLimit
          :disabled="submitting"
          :placeholder="t('chat.leave.contentPlaceholder')"
        />
      </div>
      <div class="chat-leave-actions">
        <ElButton type="primary" :loading="submitting" @click="submit">
          {{ t('chat.leave.submit') }}
        </ElButton>
      </div>
    </template>
  </div>
</template>

<script lang="ts" setup>
  import { reactive, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { ElMessage } from 'element-plus';

  defineOptions({ name: 'LeaveMessage' });

  const props = defineProps({
    /** 预填的访客昵称 */
    defaultName: String,
    /** 提交中 */
    submitting: Boolean,
    /** 是否已提交成功 */
    done: Boolean
  });

  const emit = defineEmits({
    /** 提交留言 */
    submit: (_value: {
      name: string;
      contactType: string;
      contact: string;
      content: string;
    }) => true
  });

  const { t } = useI18n();

  /** 留言表单 */
  const form = reactive({
    name: props.defaultName ?? '',
    contact: '',
    content: ''
  });

  /** 昵称变化时同步预填值, 访客改过的不覆盖 */
  watch(
    () => props.defaultName,
    (value) => {
      if (!form.name) {
        form.name = value ?? '';
      }
    }
  );

  /** 手机号 */
  const PHONE_PATTERN = /^1[3-9]\d{9}$/;
  /** 邮箱 */
  const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

  /** 按内容正则判断联系方式类型, 不匹配时返回空串 */
  const contactTypeOf = (value: string) => {
    if (PHONE_PATTERN.test(value)) {
      return 'phone';
    }
    if (EMAIL_PATTERN.test(value)) {
      return 'email';
    }
    return '';
  };

  /** 提交留言, 前后端都做格式校验 */
  const submit = () => {
    if (props.submitting) {
      return;
    }
    const contact = form.contact.trim();
    const content = form.content.trim();
    if (!contactTypeOf(contact)) {
      ElMessage.warning(t('chat.leave.contactInvalid'));
      return;
    }
    if (!content) {
      ElMessage.warning(t('chat.leave.required'));
      return;
    }
    emit('submit', {
      name: form.name.trim(),
      contactType: contactTypeOf(contact),
      contact,
      content
    });
  };
</script>

<style lang="scss" scoped>
  .chat-leave {
    padding: 12px;
    background: var(--el-bg-color);
    border-top: 1px solid var(--el-border-color-lighter);
  }

  .chat-leave-title {
    font-weight: 600;
  }

  .chat-leave-desc {
    margin: 4px 0 8px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .chat-leave-form {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .chat-leave-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 8px;
  }

  .chat-leave-done {
    display: flex;
    align-items: center;
    font-size: 13px;
    color: var(--el-color-success);
    gap: 6px;
  }

  .chat-leave-done-icon {
    font-size: 16px;
  }
</style>
