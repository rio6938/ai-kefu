<!-- 聊天气泡内容 -->
<template>
  <div class="chat-bubble" :class="`is-${role}`">
    <span v-if="role === 'ai'" class="chat-bubble-ai-badge">AI</span>
    <div v-if="quote" class="chat-bubble-quote">{{ quote }}</div>
    <LoadingBubble v-if="status === 'loading'" :text="loadingText" />
    <LoadingBubble v-else-if="typing" />
    <div v-else-if="isImage" class="chat-bubble-image">
      <ElImage
        :src="fileUrl"
        :previewSrcList="[fileUrl]"
        fit="cover"
        previewTeleported
        class="chat-bubble-image-inner"
      />
    </div>
    <div v-else-if="isFile" class="chat-bubble-file">
      <a
        class="chat-bubble-file-main"
        :href="fileUrl"
        target="_blank"
        rel="noopener"
      >
        <span class="chat-bubble-file-name">{{ fileName }}</span>
        <span class="chat-bubble-file-size">{{ fileSize }}</span>
      </a>
      <a class="chat-bubble-file-download" :href="fileUrl" download>
        {{ t('chat.file.download') }}
      </a>
    </div>
    <div v-else class="chat-bubble-content" v-html="contentHtml"></div>
  </div>
</template>

<script lang="ts" setup>
  import { computed } from 'vue';
  import type { PropType } from 'vue';
  import { useI18n } from 'vue-i18n';
  import type { ChatFileMeta } from '@/composables/use-messages';
  import { renderMarkdown } from '@/utils/markdown';
  import LoadingBubble from './loading-bubble.vue';

  defineOptions({ name: 'MessageBubble' });

  const { t } = useI18n();

  const props = defineProps({
    /** 消息内容, 附件消息存附件访问地址 */
    content: String,
    /** 发送者角色, visitor/agent/ai/system */
    role: String,
    /** 内容类型, text/image/file/system/ai_answer */
    contentType: String,
    /** 消息状态, loading 时展示加载气泡 */
    status: String,
    /** 附件消息的附加信息 */
    meta: Object as PropType<ChatFileMeta | undefined>,
    /** 引用的原消息摘要, 引用回复时有值 */
    quote: String
  });

  /** 附件地址: 跨域部署时补上接口地址前缀, 同源反代时保持不变 */
  const fileUrl = computed(() => {
    const base = (import.meta.env.VITE_API_URL ?? '').replace(/\/$/, '');
    return base + (props.content ?? '');
  });

  /** 是否是图片消息 */
  const isImage = computed(() => props.contentType === 'image');
  /** 是否是文件消息 */
  const isFile = computed(() => props.contentType === 'file');

  /** 加载气泡的提示文案 */
  const loadingText = computed(() =>
    props.role === 'ai' ? t('chat.loading.ai') : t('chat.loading.waiting')
  );

  /** AI 正在生成但还没有内容时, 气泡内只展示三点动画 */
  const typing = computed(
    () => props.role === 'ai' && props.status === 'receiving' && !props.content
  );

  /** 正文按 Markdown 渲染, 转义与链接白名单在 utils 里完成 */
  const contentHtml = computed(() => renderMarkdown(props.content ?? ''));

  /** 附件文件名, 取不到时用地址最后一段 */
  const fileName = computed(() => {
    if (props.meta?.name) {
      return props.meta.name;
    }
    const value = (props.content ?? '').split('?')[0];
    return value.slice(value.lastIndexOf('/') + 1) || t('chat.file.attachment');
  });

  /** 附件大小, 按 KB / MB 展示 */
  const fileSize = computed(() => {
    const size = props.meta?.size ?? 0;
    if (!size) {
      return '';
    }
    if (size < 1024 * 1024) {
      return `${(size / 1024).toFixed(1)} KB`;
    }
    return `${(size / 1024 / 1024).toFixed(1)} MB`;
  });
</script>

<style lang="scss" scoped>
  .chat-bubble {
    position: relative;
    display: inline-flex;
    flex-direction: column;
    max-width: 100%;
    gap: 2px;
  }

  /* 引用回复的原文, 显示在气泡顶部 */
  .chat-bubble-quote {
    margin-bottom: 6px;
    padding: 4px 8px;
    font-size: 12px;
    line-height: 18px;
    color: var(--el-text-color-secondary);
    background: var(--el-fill-color-light);
    border-left: 3px solid var(--el-border-color);
    border-radius: 2px;
    word-break: break-word;
  }

  /* AI 气泡右上角角标，不再占用头像与名字 */
  .chat-bubble-ai-badge {
    position: absolute;
    top: -9px;
    right: -8px;
    padding: 0 5px;
    font-size: 10px;
    line-height: 16px;
    color: #fff;
    background: var(--el-color-primary);
    border-radius: 6px;
  }

  .chat-bubble-content {
    font-size: 14px;
    line-height: 1.6;
    word-break: break-word;
  }

  /* Markdown 正文：气泡内保持紧凑，段落与列表不留多余空白 */
  .chat-bubble-content :deep(p) {
    margin: 0 0 4px;
  }

  .chat-bubble-content :deep(p:last-child) {
    margin-bottom: 0;
  }

  .chat-bubble-content :deep(ul),
  .chat-bubble-content :deep(ol) {
    margin: 0 0 4px;
    padding-left: 18px;
  }

  .chat-bubble-content :deep(li) {
    margin: 0 0 2px;
  }

  .chat-bubble-content :deep(a) {
    color: var(--el-color-primary);
    text-decoration: none;
  }

  .chat-bubble-content :deep(a:hover) {
    text-decoration: underline;
  }

  .chat-bubble-content :deep(code) {
    padding: 1px 4px;
    font-family: var(--el-font-family-mono, monospace);
    font-size: 12px;
    background: var(--el-fill-color-light);
    border-radius: 4px;
  }

  .chat-bubble-content :deep(pre) {
    margin: 0 0 4px;
    padding: 8px;
    overflow-x: auto;
    font-size: 12px;
    line-height: 1.5;
    background: var(--el-fill-color-light);
    border-radius: 4px;
  }

  .chat-bubble-content :deep(pre code) {
    padding: 0;
    background: none;
  }

  .chat-bubble-content :deep(blockquote) {
    margin: 0 0 4px;
    padding-left: 8px;
    color: var(--el-text-color-secondary);
    border-left: 2px solid var(--el-border-color);
  }

  .chat-bubble-content :deep(table) {
    max-width: 100%;
    margin: 0 0 4px;
    font-size: 13px;
    border-collapse: collapse;
  }

  .chat-bubble-content :deep(th),
  .chat-bubble-content :deep(td) {
    padding: 2px 8px;
    border: 1px solid var(--el-border-color-lighter);
  }

  .chat-bubble-content :deep(th) {
    font-weight: 600;
    background: var(--el-fill-color-light);
  }

  .chat-bubble-content :deep(h1),
  .chat-bubble-content :deep(h2),
  .chat-bubble-content :deep(h3),
  .chat-bubble-content :deep(h4),
  .chat-bubble-content :deep(h5),
  .chat-bubble-content :deep(h6) {
    margin: 0 0 4px;
    font-size: 14px;
    font-weight: 600;
  }

  .chat-bubble-content :deep(hr) {
    margin: 6px 0;
    border: none;
    border-top: 1px solid var(--el-border-color);
  }

  .chat-bubble-content :deep(img) {
    max-width: 100%;
    border-radius: 4px;
  }

  .chat-bubble.is-system .chat-bubble-content {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .chat-bubble-image {
    max-width: 220px;
  }

  .chat-bubble-image-inner {
    display: block;
    width: 100%;
    max-height: 220px;
    border-radius: 4px;
    overflow: hidden;
  }

  .chat-bubble-file {
    display: flex;
    align-items: center;
    max-width: 240px;
    gap: 10px;
  }

  .chat-bubble-file-main {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    min-width: 0;
    color: inherit;
    text-decoration: none;
    gap: 2px;
  }

  .chat-bubble-file-name {
    overflow: hidden;
    font-size: 14px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chat-bubble-file-size {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .chat-bubble-file-download {
    flex: none;
    font-size: 12px;
    color: var(--el-color-primary);
    text-decoration: none;
  }
</style>
