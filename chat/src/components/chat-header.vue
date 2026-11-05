<!-- 聊天窗顶部 -->
<template>
  <div class="chat-header">
    <ElDropdown trigger="click" @command="handleCommand">
      <div class="chat-header-logo">
        <ElAvatar :size="34" :src="avatar">{{ logoText }}</ElAvatar>
      </div>
      <template #dropdown>
        <ElDropdownMenu>
          <ElDropdownItem command="profile">
            {{ t('chat.header.editProfile') }}
          </ElDropdownItem>
        </ElDropdownMenu>
      </template>
    </ElDropdown>
    <div class="chat-header-main">
      <div class="chat-header-title">{{ title }}</div>
      <div class="chat-header-subtitle">
        <span class="chat-header-status" :class="statusClass" />
        <span>{{ subtitle }}</span>
      </div>
    </div>
    <ElTag v-if="aiMode === 1" size="small" type="primary" effect="plain">
      {{ t('chat.header.aiMode') }}
    </ElTag>
    <ElButton v-else-if="agentName" size="small" text>
      {{ agentName }}
    </ElButton>
    <ElDropdown trigger="click" @command="handleLocale">
      <ElButton size="small" text :title="t('chat.header.language')">
        <svg class="chat-header-globe" viewBox="0 0 24 24" aria-hidden="true">
          <circle
            cx="12"
            cy="12"
            r="9"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
          />
          <ellipse
            cx="12"
            cy="12"
            rx="4"
            ry="9"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
          />
          <path
            d="M3.5 9h17M3.5 15h17"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
          />
        </svg>
      </ElButton>
      <template #dropdown>
        <ElDropdownMenu>
          <ElDropdownItem
            v-for="item in localeOptions"
            :key="item.value"
            :command="item.value"
            :disabled="item.value === locale"
          >
            {{ t(item.labelKey) }}
          </ElDropdownItem>
        </ElDropdownMenu>
      </template>
    </ElDropdown>
    <ElButton
      v-if="embed"
      size="small"
      text
      :title="t('chat.header.collapse')"
      @click="emit('close')"
    >
      {{ t('chat.header.collapse') }}
    </ElButton>
  </div>
</template>

<script lang="ts" setup>
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { locale, localeOptions, setLocale } from '@/i18n';

  defineOptions({ name: 'ChatHeader' });

  const props = defineProps({
    /** 渠道名称 */
    channelName: String,
    /** 接待客服名称 */
    agentName: String,
    /** AI 自动回复开关, 0关闭, 1开启 */
    aiMode: Number,
    /** 会话状态, 0待接待, 1 AI 进行中, 2已结束, 3留言, 4人工进行中 */
    sessionStatus: Number,
    /** 排队名次, 1 起, 只在待接待时有值 */
    queuePosition: Number,
    /** 预计等待秒数, 只在待接待时有值 */
    estimatedWaitSeconds: Number,
    /** 头像地址 */
    avatar: String,
    /** 是否嵌入在悬浮组件里, 嵌入时显示收起按钮 */
    embed: Boolean
  });

  const emit = defineEmits({
    /** 收起面板 */
    close: () => true,
    /** 打开修改资料 */
    profile: () => true,
    /** 切换语言 */
    localeChange: (_locale: string) => true
  });

  const { t } = useI18n();

  /** 头像菜单选择 */
  const handleCommand = (command: string) => {
    if (command === 'profile') {
      emit('profile');
    }
  };

  /** 语言下拉选择, 切换后通知上层同步给悬浮脚本 */
  const handleLocale = (command: string) => {
    const next = setLocale(command);
    if (next !== command) {
      return;
    }
    emit('localeChange', next);
  };

  /** 标题 */
  const title = computed(() => {
    if (props.agentName) {
      return t('chat.header.serving', { name: props.agentName });
    }
    return (
      props.channelName ||
      import.meta.env.VITE_APP_NAME ||
      t('chat.header.title')
    );
  });

  /** 头像文字 */
  const logoText = computed(() => title.value.slice(0, 1));

  /** 状态文案 */
  const subtitle = computed(() => {
    switch (props.sessionStatus) {
      case 0:
        return queueSubtitle();
      case 2:
        return t('chat.header.status.ended');
      case 3:
        return t('chat.header.status.leave');
      case 4:
        return t('chat.header.status.human');
      default:
        return t('chat.header.status.ai');
    }
  });

  /** 排队文案: 第一位用「马上轮到您」, 其余展示前面人数与预计分钟 */
  const queueSubtitle = () => {
    const position = props.queuePosition ?? 0;
    if (position <= 0) {
      return t('chat.header.status.queue');
    }
    if (position === 1) {
      return t('chat.header.status.queueNext');
    }
    const seconds = props.estimatedWaitSeconds ?? 0;
    const minutes = Math.max(1, Math.round(seconds / 60));
    return t('chat.header.status.queuePosition', {
      position: position - 1,
      minutes
    });
  };

  /** 状态点样式 */
  const statusClass = computed(() =>
    props.sessionStatus === 2 ? 'is-offline' : 'is-online'
  );
</script>

<style lang="scss" scoped>
  .chat-header {
    display: flex;
    align-items: center;
    padding: 12px 16px;
    background: var(--el-bg-color);
    gap: 10px;
    border-bottom: 1px solid var(--el-border-color-lighter);
  }

  .chat-header-logo {
    display: flex;
    flex: none;
  }

  .chat-header-main {
    flex: 1 1 auto;
    min-width: 0;
  }

  .chat-header-title {
    overflow: hidden;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chat-header-subtitle {
    display: flex;
    align-items: center;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    gap: 6px;
  }

  .chat-header-status {
    width: 6px;
    height: 6px;
    border-radius: 50%;

    &.is-online {
      background: var(--el-color-success);
    }

    &.is-offline {
      background: var(--el-color-info);
    }
  }

  /* 语言下拉图标, 内联 SVG 避免为单个图标引依赖 */
  .chat-header-globe {
    width: 16px;
    height: 16px;
  }
</style>
