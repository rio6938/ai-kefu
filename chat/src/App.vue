<!-- 聊天窗入口, 按当前语言给 Element Plus 下发语言包 -->
<template>
  <ElConfigProvider :locale="elementLocale">
    <ChatWindow />
  </ElConfigProvider>
</template>

<script lang="ts" setup>
  import { computed } from 'vue';
  import { ElConfigProvider } from 'element-plus';
  import zhCn from 'element-plus/es/locale/lang/zh-cn';
  import en from 'element-plus/es/locale/lang/en';
  import ChatWindow from '@/views/ChatWindow.vue';
  import { locale } from '@/i18n';

  defineOptions({ name: 'App' });

  /** Element Plus 的语言包, 只覆盖面板用到的两种语言 */
  const locales: Record<string, typeof zhCn> = {
    'zh-CN': zhCn,
    en
  };

  /** 当前语言对应的 Element Plus 语言包, 未知语言回退简体中文 */
  const elementLocale = computed(() => locales[locale.value] ?? zhCn);
</script>
