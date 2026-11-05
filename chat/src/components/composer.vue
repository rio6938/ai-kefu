<!-- 聊天窗输入区 -->
<template>
  <div class="chat-composer">
    <QuickReplies :items="quickReplies" @select="handleSelect" />
    <ElInput
      type="textarea"
      resize="none"
      :rows="2"
      :maxlength="1000"
      :disabled="disabled"
      :placeholder="placeholder"
      v-model="content"
      @keydown.enter.exact.prevent="submit"
    />
    <div class="chat-composer-footer">
      <div class="chat-composer-tools">
        <ElButton
          v-if="showHandoff"
          text
          :disabled="sending"
          @click="handleHandoff"
        >
          {{ t('chat.composer.handoff') }}
        </ElButton>
        <ElTooltip
          v-if="fileEnabled"
          :content="t('chat.composer.attachmentTooltip')"
          placement="top"
        >
          <ElUpload
            action=""
            :showFileList="false"
            :accept="accept"
            :disabled="disabled || uploading"
            :beforeUpload="checkFile"
            :httpRequest="handleHttpRequest"
            class="chat-composer-upload"
          >
            <ElButton text :loading="uploading" :disabled="disabled">
              {{ t('chat.composer.attachment') }}
            </ElButton>
          </ElUpload>
        </ElTooltip>
        <span class="chat-composer-tip">{{ tip }}</span>
      </div>
      <ElButton
        type="primary"
        :loading="sending"
        :disabled="disabled"
        @click="submit"
      >
        {{ t('chat.composer.send') }}
      </ElButton>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { computed } from 'vue';
  import type { PropType } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { ElMessage, ElUpload } from 'element-plus';
  import type { UploadRequestOptions } from 'element-plus';
  import QuickReplies from '@/components/quick-replies.vue';
  import type { QuickReplyItem } from '@/api/types';

  defineOptions({ name: 'ChatComposer' });

  const props = defineProps({
    /** 输入框内容 */
    modelValue: String,
    /** 发送中 */
    sending: Boolean,
    /** 附件上传中 */
    uploading: Boolean,
    /** 是否禁用输入 */
    disabled: Boolean,
    /** 是否必须登录后咨询 */
    needLogin: Number,
    /** 会话状态 */
    sessionStatus: Number,
    /** 访客附件上传开关 */
    fileEnabled: Boolean,
    /** 访客端「转人工」按钮开关 */
    handoffEnabled: Boolean,
    /** 单文件大小上限, 单位 MB */
    fileMaxSizeMb: Number,
    /** 允许上传的附件后缀, 不含点号 */
    fileAllowedExt: Array as PropType<string[]>,
    /** 访客快捷问题 */
    quickReplies: {
      type: Array as PropType<QuickReplyItem[]>,
      default: () => []
    }
  });

  const emit = defineEmits({
    /** 输入框内容变化 */
    'update:modelValue': (_value: string) => true,
    /** 发送消息 */
    send: (_content: string) => true,
    /** 选中快捷问题, 直接发送 */
    quickReply: (_item: QuickReplyItem) => true,
    /** 选择附件, 由上层完成上传与发送 */
    upload: (_file: File) => true,
    /** 请求转人工, 由上层校验并调用接口 */
    handoff: () => true
  });

  const { t } = useI18n();

  /** 输入框内容 */
  const content = computed({
    get: () => props.modelValue ?? '',
    set: (value: string) => emit('update:modelValue', value)
  });

  /** 输入框提示 */
  const placeholder = computed(() => {
    if (props.disabled) {
      return props.sessionStatus === 2
        ? t('chat.composer.ended')
        : t('chat.composer.disabled');
    }
    return t('chat.composer.placeholder');
  });

  /** 底部提示 */
  const tip = computed(() =>
    props.needLogin === 1
      ? t('chat.composer.needLogin')
      : t('chat.composer.aiTip')
  );

  /** 文件选择的 accept, 取公开配置里的后缀白名单 */
  const accept = computed(() =>
    (props.fileAllowedExt ?? []).map((item) => '.' + item).join(',')
  );

  /** 是否展示「转人工」按钮: 只在 AI 进行中并且渠道开了开关时展示 */
  const showHandoff = computed(
    () => props.handoffEnabled && props.sessionStatus === 1 && !props.disabled
  );

  /** 点击「转人工」, 具体确认与接口调用由上层完成 */
  const handleHandoff = () => {
    if (props.sending) {
      return;
    }
    emit('handoff');
  };

  /** 提交文本 */
  const submit = () => {
    const value = content.value.trim();
    if (!value || props.disabled || props.sending) {
      return;
    }
    emit('send', value);
  };

  /** 选中快捷问题后直接发送 */
  const handleSelect = (item: QuickReplyItem) => {
    if (props.disabled || props.sending) {
      return;
    }
    emit('quickReply', item);
  };

  /** 选择文件时先按公开配置做前端校验, 不通过直接拦下 */
  const checkFile = (file: File) => {
    const ext = file.name.slice(file.name.lastIndexOf('.') + 1).toLowerCase();
    const allowed = props.fileAllowedExt ?? [];
    if (allowed.length && !allowed.includes(ext)) {
      ElMessage.warning(
        t('chat.file.unsupported', { ext: allowed.join('/') })
      );
      return false;
    }
    const maxSizeMb = props.fileMaxSizeMb ?? 0;
    if (maxSizeMb > 0 && file.size > maxSizeMb * 1024 * 1024) {
      ElMessage.warning(t('chat.file.tooLarge', { size: maxSizeMb }));
      return false;
    }
    return true;
  };

  /** 自定义上传: 具体上传由上层完成, 组件内不请求 action 也不维护文件列表 */
  const handleHttpRequest = (options: UploadRequestOptions) => {
    emit('upload', options.file as File);
    options.onSuccess(void 0);
    return Promise.resolve();
  };
</script>

<style lang="scss" scoped>
  .chat-composer {
    padding: 10px 12px;
    background: var(--el-bg-color);
    border-top: 1px solid var(--el-border-color-lighter);
  }

  .chat-composer-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 8px;
    gap: 8px;
  }

  .chat-composer-tools {
    display: flex;
    flex: 1 1 auto;
    align-items: center;
    min-width: 0;
    gap: 8px;
  }

  .chat-composer-upload {
    flex: none;
    line-height: 1;
  }

  .chat-composer-tip {
    overflow: hidden;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
