<!-- 客户端聊天窗 -->
<template>
  <div class="chat-window" :style="themeStyle">
    <ChatHeader
      :channelName="config?.name"
      :agentName="agentName"
      :aiMode="session?.aiMode ?? result?.aiMode"
      :sessionStatus="session?.status ?? result?.sessionStatus"
      :queuePosition="session?.queuePosition ?? 0"
      :estimatedWaitSeconds="session?.estimatedWaitSeconds ?? 0"
      :embed="embed"
      @close="handleClose"
      @profile="profileVisible = true"
      @localeChange="handleLocaleChange"
    />
    <div class="chat-window-body">
      <div v-if="helpEnabled" class="chat-window-tabs">
        <ElRadioGroup v-model="activeTab" size="small">
          <ElRadioButton value="chat">
            {{ t('chat.tabs.chat') }}
          </ElRadioButton>
          <ElRadioButton value="help">
            {{ t('chat.tabs.help') }}
          </ElRadioButton>
        </ElRadioGroup>
      </div>
      <PrechatForm
        v-if="needProfile"
        :profile="profile"
        :loading="loading"
        :showSkip="true"
        @submit="handleSubmitProfile"
        @skip="skipProfile"
      />
      <ElResult
        v-else-if="error"
        icon="error"
        :title="t('chat.session.connectFailed')"
        :subTitle="error"
      >
        <template #extra>
          <ElButton type="primary" @click="reload">
            {{ t('chat.session.retry') }}
          </ElButton>
        </template>
      </ElResult>
      <div v-else-if="activeTab === 'help'" class="chat-window-help">
        <div
          v-for="item in quickReplies"
          :key="item.quickReplyId"
          class="chat-window-help-item"
          @click="handleHelpSelect(item)"
        >
          <div class="chat-window-help-title">{{ item.title }}</div>
          <div class="chat-window-help-content">{{ item.content }}</div>
        </div>
        <ElEmpty
          v-if="!quickReplies.length"
          :description="t('chat.help.empty')"
          :imageSize="60"
        />
      </div>
      <div v-else ref="messagesRef" class="chat-window-messages">
        <div
          v-for="item in messages"
          :key="item.key"
          class="chat-window-message"
          :class="`is-${item.fromRole ?? 'visitor'}`"
        >
          <div
            v-if="item.fromRole !== 'ai'"
            class="chat-window-message-header"
          >
            {{ item.header }}
          </div>
          <MessageBubble
            :content="item.content"
            :role="item.fromRole"
            :contentType="item.contentType"
            :status="item.status"
            :meta="item.meta"
            :quote="item.quoteContent"
          />
          <div
            v-if="showHandoffHint(item.key)"
            class="chat-window-handoff-hint"
          >
            {{ t('chat.handoff.transferred') }}
          </div>
        </div>
        <div
          v-if="loading || waitingReply"
          class="chat-window-message is-agent"
        >
          <div class="chat-window-message-header">
            {{ t('chat.message.agent') }}
          </div>
          <LoadingBubble class="chat-bubble" :text="loadingText" />
        </div>
        <ElEmpty
          v-if="!messages.length && !loading && !waitingReply"
          :description="emptyText"
          :imageSize="60"
        />
        <div v-if="agentTyping" class="chat-window-typing">
          <span class="chat-window-typing-dots">
            <i></i>
            <i></i>
            <i></i>
          </span>
          {{ t('chat.typing.agent') }}
        </div>
      </div>
    </div>
    <template v-if="!needProfile">
      <LeaveMessage
        v-if="showLeaveCard"
        :defaultName="result?.nickname ?? ''"
        :submitting="leaveSubmitting"
        :done="leaveDone"
        @submit="handleLeaveSubmit"
      />
      <SessionRate
        v-if="showRateCard"
        :submitting="rateSubmitting"
        @submit="handleRateSubmit"
        @skip="handleRateSkip"
      />
      <Composer
        v-else
        v-model="content"
        :sending="sending"
        :uploading="uploading"
        :disabled="!canSend"
        :needLogin="config?.needLogin ?? 0"
        :sessionStatus="session?.status ?? result?.sessionStatus ?? 0"
        :fileEnabled="config?.fileEnabled ?? false"
        :fileMaxSizeMb="config?.fileMaxSizeMb ?? 0"
        :fileAllowedExt="config?.fileAllowedExt ?? []"
        :handoffEnabled="handoffEnabled"
        :quickReplies="quickReplies"
        @send="handleSend"
        @quick-reply="handleQuickReply"
        @upload="handleUpload"
        @handoff="handleHandoff"
      />
      <div v-if="sessionClosed" class="chat-window-reconsult">
        <ElButton
          type="primary"
          plain
          :loading="reconsulting"
          @click="handleReconsult"
        >
          {{ t('chat.rate.again') }}
        </ElButton>
      </div>
    </template>
    <ElDialog
      :width="360"
      :title="t('chat.prechat.editTitle')"
      v-model="profileVisible"
      :closeOnClickModal="false"
    >
      <PrechatForm
        :profile="profile"
        :loading="profileLoading"
        :showSkip="false"
        @submit="handleUpdateProfile"
      />
    </ElDialog>
  </div>
</template>

<script lang="ts" setup>
  import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { ElMessageBox } from 'element-plus';
  import type { EventNotification, RecvMessage } from 'easyjssdk';
  import ChatHeader from '@/components/chat-header.vue';
  import MessageBubble from '@/components/message-bubble.vue';
  import LoadingBubble from '@/components/loading-bubble.vue';
  import Composer from '@/components/composer.vue';
  import LeaveMessage from '@/components/leave-message.vue';
  import SessionRate from '@/components/session-rate.vue';
  import PrechatForm from '@/components/prechat-form.vue';
  import { useBootstrap } from '@/composables/use-bootstrap';
  import { useMessages } from '@/composables/use-messages';
  import type { ChatMessageItem } from '@/composables/use-messages';
  import { useStream } from '@/composables/use-stream';
  import { ChatImClient, createClientMsgNo } from '@/im/client';
  import { greetingKey, locale as currentLocale, setLocale } from '@/i18n';
  import {
    bootstrap,
    getSession,
    handoffSession,
    rateSession,
    readSession,
    sendTyping
  } from '@/api/session';
  import { submitLeaveMessage } from '@/api/leave-message';
  import { listQuickReplies } from '@/api/quick-reply';
  import { uploadVisitorFile } from '@/api/file';
  import type {
    ChatPayload,
    QuickReplyItem,
    Session,
    UploadedFile,
    VisitorProfile
  } from '@/api/types';

  defineOptions({ name: 'ChatWindow' });

  /** 会话状态: 待接待与人工进行中, 与服务端 model.KefuSessionStatus 一致 */
  const STATUS_QUEUEING = 0;
  const STATUS_CLOSED = 2;
  const STATUS_LEAVE_MESSAGE = 3;
  const STATUS_HUMAN = 4;

  /** 排队轮询间隔, 只在待接待态运行 */
  const QUEUE_POLL_INTERVAL = 15000;

  const { t } = useI18n();

  /** 页面查询参数 */
  const search = new URLSearchParams(window.location.search);

  /** 是否以悬浮面板方式嵌入, 嵌入时不显示整页外壳并上报未读 */
  const embed = search.get('embed') === 'widget';

  /** 父窗口域名, postMessage 的 targetOrigin 不用 * */
  const parentOrigin = (() => {
    const referrer = document.referrer;
    if (referrer) {
      try {
        return new URL(referrer).origin;
      } catch {
        return '';
      }
    }
    const ancestors = window.location.ancestorOrigins;
    return ancestors && ancestors.length ? ancestors[0] : '';
  })();

  /** 向父窗口发消息, 只在嵌入模式且有父窗口域名时生效 */
  const postToParent = (type: string, data?: Record<string, unknown>) => {
    if (!embed || !parentOrigin || window.parent === window) {
      return;
    }
    window.parent.postMessage({ type, data: data ?? {} }, parentOrigin);
  };

  /** 访客接入 */
  const {
    channelCode,
    config,
    result,
    loading,
    error,
    profile,
    needProfile,
    reload,
    submitProfile,
    skipProfile,
    updateProfile,
    identify
  } = useBootstrap();

  /** 会话详情, 轮询与重连后刷新 */
  const session = ref<Session | null>(null);

  /** 访客昵称 */
  const visitorName = ref(t('chat.message.self'));

  /** 消息列表 */
  const {
    messages,
    sending,
    append,
    replace,
    loadHistory,
    reset,
    refreshHeader,
    fromPayload
  } = useMessages(visitorName);

  /** AI 流式事件 */
  const stream = useStream(messages);

  /** 输入框内容 */
  const content = ref('');

  /** 是否开启帮助页签, 由悬浮脚本的 data-tabs 控制 */
  const helpEnabled = computed(() =>
    (search.get('tabs') ?? '')
      .split(',')
      .map((item) => item.trim())
      .includes('help')
  );

  /** 当前页签, chat 对话, help 帮助 */
  const activeTab = ref<'chat' | 'help'>('chat');

  /** 点击帮助页签的问题, 切回对话并直接发送 */
  const handleHelpSelect = (item: QuickReplyItem) => {
    activeTab.value = 'chat';
    handleQuickReply(item);
  };

  /** 访客快捷问题 */
  const quickReplies = ref<QuickReplyItem[]>([]);

  /** 附件上传中 */
  const uploading = ref(false);

  /** IM 客户端 */
  const imClient = new ChatImClient();

  /** 消息区容器, 新消息后滚到底部 */
  const messagesRef = ref<HTMLElement | null>(null);

  /** 空消息提示, 问候语按本地时间与当前语言生成 */
  const emptyText = computed(() =>
    t('chat.session.empty', { greeting: t(greetingKey()) })
  );

  /** 访客发出消息后等待客服或 AI 回复, 展示加载气泡 */
  const waitingReply = ref(false);

  /** 等待回复超过一段时间后换成安抚文案 */
  const waitLong = ref(false);

  /** 等待回复的定时器 */
  let waitTimer: ReturnType<typeof setTimeout> | undefined;

  /** 加载气泡的提示文案 */
  const loadingText = computed(() => {
    if (loading.value) {
      return t('chat.session.connecting');
    }
    return waitLong.value
      ? t('chat.loading.waitingLong')
      : t('chat.loading.waiting');
  });

  /** 开始等待回复, 超时后换安抚文案, 避免一直显示正在回复 */
  const startWaitingReply = () => {
    waitingReply.value = true;
    waitLong.value = false;
    if (waitTimer) {
      clearTimeout(waitTimer);
    }
    waitTimer = setTimeout(() => {
      waitLong.value = true;
    }, 30000);
  };

  /** 结束等待回复 */
  const stopWaitingReply = () => {
    waitingReply.value = false;
    waitLong.value = false;
    if (waitTimer) {
      clearTimeout(waitTimer);
      waitTimer = undefined;
    }
  };

  /** 滚动到最新一条消息 */
  const scrollToBottom = () => {
    const el = messagesRef.value;
    if (el) {
      el.scrollTop = el.scrollHeight;
    }
  };

  /** 消息变化或流式增量到达时滚动到底部 */
  watch(messages, () => nextTick(scrollToBottom), { deep: true });

  /** 主题色 */
  const themeStyle = computed(() => ({
    '--el-color-primary': config.value?.themeColor || '#409eff'
  }));

  /** 接待客服名称: 优先取会话详情里回填的客服, 拿不到时用接入结果 */
  const agentName = computed(() => {
    const session1 = session.value;
    if (!session1?.agentId) {
      return '';
    }
    return (
      session1.agent?.nickname ||
      session1.agent?.username ||
      result.value?.agentName ||
      ''
    );
  });

  /** 已从 AI 转给人工: 待接待与人工进行中都属于已转接 */
  const handedOffToHuman = computed(() => {
    const status = session.value?.status;
    return status === STATUS_QUEUEING || status === STATUS_HUMAN;
  });

  /** 最后一条 AI 消息的标识, 转人工后在它下面补一句灰字 */
  const lastAiKey = computed(() => {
    for (let i = messages.value.length - 1; i >= 0; i -= 1) {
      if (messages.value[i].fromRole === 'ai') {
        return messages.value[i].key;
      }
    }
    return '';
  });

  /** 是否在指定气泡下方展示「已转接人工客服」 */
  const showHandoffHint = (key: string) =>
    handedOffToHuman.value && lastAiKey.value === key;

  /** 是否展示「转人工」按钮, 以渠道公开配置为准 */
  const handoffEnabled = computed(() => config.value?.handoffEnabled !== false);

  /** 转人工请求中, 避免连续点击重复提交 */
  const handoffLoading = ref(false);

  /** 当前会话已转人工, 不再重复调用接口 */
  const handoffDone = ref(false);

  /** 转人工时没有客服在线, 直接展开留言卡片 */
  const leaveForced = ref(false);

  /** 留言提交中 */
  const leaveSubmitting = ref(false);

  /** 留言已提交成功 */
  const leaveDone = ref(false);

  /** 是否展开留言卡片: 会话转留言态或转人工时无人在线 */
  const showLeaveCard = computed(() => {
    if (needProfile.value) {
      return false;
    }
    const status = session.value?.status ?? result.value?.sessionStatus;
    return status === STATUS_LEAVE_MESSAGE || leaveForced.value;
  });

  /** 会话已结束 */
  const sessionClosed = computed(
    () => (session.value?.status ?? result.value?.sessionStatus) === STATUS_CLOSED
  );

  /** 评价提交中 */
  const rateSubmitting = ref(false);

  /** 访客点了「暂不评价」, 本次会话不再展示评价卡 */
  const rateSkipped = ref(false);

  /** 再次咨询请求中 */
  const reconsulting = ref(false);

  /**
   * 是否展示评价卡: 会话已结束、评价开关开启、尚未评价且未跳过
   */
  const showRateCard = computed(
    () =>
      sessionClosed.value &&
      config.value?.rateEnabled === true &&
      !(session.value?.rateScore ?? 0) &&
      !rateSkipped.value
  );

  /** 是否可以发送消息 */
  const canSend = computed(() => {
    if (!session.value || !result.value) {
      return false;
    }
    // 留言提交后与留言态的会话不再允许发消息
    if (leaveDone.value) {
      return false;
    }
    return (
      session.value.status !== STATUS_CLOSED &&
      session.value.status !== STATUS_LEAVE_MESSAGE
    );
  });

  /** 当前未读数, 嵌入模式下上报给悬浮脚本 */
  const unreadCount = ref(0);

  /** 上报未读数 */
  const reportUnread = () => {
    postToParent('kefu:unread', { count: unreadCount.value });
  };

  /** 气泡可见时清零未读 */
  const clearUnread = () => {
    if (unreadCount.value === 0) {
      return;
    }
    unreadCount.value = 0;
    reportUnread();
  };

  /** 面板是否处于展开状态, 收起时到达的消息计入未读 */
  const panelVisible = ref(true);

  /** 收到 IM 消息 */
  const handleImMessage = (message: RecvMessage) => {
    const payload = (message.payload ?? {}) as ChatPayload;
    if (!payload.sessionId) {
      return;
    }
    // 「正在输入」不落屏, 只刷新对方输入提示
    if (payload.type === 'typing') {
      if (payload.role === 'agent') {
        markAgentTyping();
      }
      return;
    }
    append(fromPayload(payload, message.clientMsgNo));
    // 客服或 AI 已回复, 收起等待中的加载气泡
    if (payload.role !== 'visitor') {
      stopWaitingReply();
    }
    // 只刷新会话状态与已读，不重载历史：
    // 重载会把尚未收尾的 AI 流式气泡整个替掉，导致客户看到消息“卡住后成批出现”
    if (payload.role !== 'visitor') {
      refreshSessionMeta(payload.sessionId);
      if (
        embed &&
        (!panelVisible.value || document.visibilityState !== 'visible')
      ) {
        unreadCount.value += 1;
        reportUnread();
      }
    }
  };

  /** 刷新会话状态与已读，不动消息列表；
      会话刚转成待接待或人工接待时补拉一次历史，
      避免推送丢失（如 IM 频道日志冲突）时客户看不到 AI 的收尾话术 */
  const refreshSessionMeta = async (sessionId: number) => {
    if (!result.value) {
      return;
    }
    try {
      const before = session.value?.status;
      session.value = await getSession(sessionId, result.value.visitorKey);
      await readSession(sessionId, result.value.visitorKey);
      const after = session.value?.status;
      if (
        before !== void 0 &&
        after !== before &&
        (after === STATUS_QUEUEING || after === STATUS_HUMAN)
      ) {
        await loadHistory(sessionId, result.value.visitorKey);
      }
      // 刚进入待接待时立即提示排队位置, 不用等下一轮轮询
      applyQueueHint();
    } catch {
      // 刷新失败不影响已上屏的消息
    }
  };

  /** 排队轮询定时器, 只在待接待态运行 */
  let queueTimer: ReturnType<typeof setInterval> | undefined;

  /** 上一次提示过的排队名次, 名次变化时才补系统气泡 */
  let lastQueuePosition = 0;

  /** 清掉排队轮询 */
  const stopQueuePolling = () => {
    if (queueTimer) {
      clearInterval(queueTimer);
      queueTimer = undefined;
    }
  };

  /** 排队名次变化时补一条系统气泡, 文案随当前语言 */
  const applyQueueHint = () => {
    const current = session.value;
    if (!current || current.status !== STATUS_QUEUEING) {
      return;
    }
    const position = current.queuePosition ?? 0;
    if (position <= 0 || position === lastQueuePosition) {
      return;
    }
    lastQueuePosition = position;
    const minutes = Math.max(
      1,
      Math.round((current.estimatedWaitSeconds ?? 0) / 60)
    );
    appendSystemBubble(
      `queue_${position}`,
      position === 1
        ? t('chat.handoff.queueNext')
        : t('chat.handoff.queueChanged', { position: position - 1, minutes })
    );
  };

  /** 轮询会话详情刷新排队位置, 只拉详情不拉消息; 页面隐藏时不请求 */
  const pollQueue = async () => {
    const sessionId = session.value?.sessionId ?? result.value?.sessionId;
    if (!sessionId || !result.value) {
      return;
    }
    if (document.visibilityState !== 'visible') {
      return;
    }
    try {
      session.value = await getSession(sessionId, result.value.visitorKey);
    } catch {
      return;
    }
    applyQueueHint();
    if (session.value.status !== STATUS_QUEUEING) {
      stopQueuePolling();
    }
  };

  /** 按会话状态启停排队轮询: 只在待接待态运行 */
  const syncQueuePolling = () => {
    const status = session.value?.status ?? result.value?.sessionStatus;
    if (status !== STATUS_QUEUEING) {
      stopQueuePolling();
      lastQueuePosition = 0;
      return;
    }
    if (queueTimer) {
      return;
    }
    queueTimer = setInterval(pollQueue, QUEUE_POLL_INTERVAL);
  };

  /** 会话状态变化时启停排队轮询 */
  watch(() => session.value?.status, syncQueuePolling);

  /** 客服是否正在输入 */
  const agentTyping = ref(false);

  /** 对方输入提示的隐藏定时器, 5s 内没再收到就隐藏 */
  let typingTimer: ReturnType<typeof setTimeout> | undefined;

  /** 收到对方输入事件后展示提示并续期自动隐藏 */
  const markAgentTyping = () => {
    agentTyping.value = true;
    if (typingTimer) {
      clearTimeout(typingTimer);
    }
    typingTimer = setTimeout(() => {
      agentTyping.value = false;
      typingTimer = undefined;
    }, 5000);
  };

  /** 清掉输入提示与定时器 */
  const clearAgentTyping = () => {
    agentTyping.value = false;
    if (typingTimer) {
      clearTimeout(typingTimer);
      typingTimer = undefined;
    }
  };

  /** 上报「正在输入」的节流时间, 3s 内只上报一次 */
  const TYPING_THROTTLE = 3000;

  /** 上次上报正在输入的时间 */
  let lastTypingAt = 0;

  /** 输入内容变化时节流上报正在输入, 只对已接入的会话生效 */
  watch(content, (value) => {
    if (!value || !value.trim()) {
      return;
    }
    const sessionId = session.value?.sessionId ?? result.value?.sessionId;
    if (!sessionId || !result.value) {
      return;
    }
    const now = Date.now();
    if (now - lastTypingAt < TYPING_THROTTLE) {
      return;
    }
    lastTypingAt = now;
    sendTyping(sessionId, result.value.visitorKey).catch(() => {
      // 输入状态上报失败不影响正常发消息
    });
  });

  /** 收到 IM 自定义事件, AI 流式增量 */
  const handleImEvent = (event: EventNotification) => {
    // AI 已开始或结束输出, 收起等待气泡, 由 AI 气泡展示加载动画
    if (
      event.type === 'stream.open' ||
      event.type === 'stream.delta' ||
      event.type === 'stream.finish' ||
      event.type === 'stream.cancel' ||
      event.type === 'stream.error'
    ) {
      stopWaitingReply();
    }
    stream.apply(event);
  };

  /** 刷新会话状态与历史消息, 重连后恢复终态 */
  const refreshSession = async (sessionId: number) => {
    if (!result.value) {
      return;
    }
    try {
      session.value = await getSession(sessionId, result.value.visitorKey);
      await loadHistory(sessionId, result.value.visitorKey);
      await readSession(sessionId, result.value.visitorKey);
      // 最后一条还是访客发的, 说明客服尚未回复, 继续展示加载气泡
      const last = messages.value[messages.value.length - 1];
      const status = session.value?.status;
      if (last && last.fromRole === 'visitor' && status !== STATUS_CLOSED) {
        startWaitingReply();
      } else {
        stopWaitingReply();
      }
      // 刷新页面时如果仍在排队, 立即提示排队位置
      applyQueueHint();
    } catch {
      // 刷新失败不影响已上屏的消息
    }
  };

  /** 修改资料弹窗显示状态 */
  const profileVisible = ref(false);

  /** 修改资料提交中 */
  const profileLoading = ref(false);

  /** 提交预填资料 */
  const handleSubmitProfile = (value: VisitorProfile) => {
    submitProfile(value).catch(() => {
      // 接入失败时页面已展示错误与重试入口
    });
  };

  /** 修改资料后重新接入, 让工作台拿到最新的昵称与邮箱 */
  const handleUpdateProfile = (value: VisitorProfile) => {
    profileLoading.value = true;
    updateProfile(value)
      .then(() => {
        profileVisible.value = false;
      })
      .catch(() => {
        // 失败时保留弹窗，用户可重新提交
      })
      .finally(() => {
        profileLoading.value = false;
      });
  };

  /** 追加一条系统错误气泡, 文案随当前语言 */
  const appendSystemError = (key: string, reason?: string) => {
    append({
      key: `error_${Date.now()}`,
      content: t(key, { reason: reason || t('chat.error.retry') }),
      header: t('chat.message.system'),
      placement: 'start',
      status: 'error',
      fromRole: 'system'
    });
  };

  /** 追加一条系统气泡, 文案已翻译好, key 相同不会重复上屏 */
  const appendSystemBubble = (key: string, content: string) => {
    append({
      key,
      content,
      header: t('chat.message.system'),
      placement: 'start',
      status: 'finish',
      fromRole: 'system'
    });
  };

  /** 追加欢迎语气泡: 欢迎语为空时用默认问候语兜底, 同一会话只追加一次 */
  const appendWelcome = (sessionId: number) => {
    const text =
      (result.value?.welcomeText ?? '').trim() || emptyText.value.trim();
    if (!text) {
      return;
    }
    appendSystemBubble(`welcome_${sessionId}`, text);
  };

  /** 提交留言, 成功后卡片换成已收到的提示 */
  const handleLeaveSubmit = async (value: {
    name: string;
    contactType: string;
    contact: string;
    content: string;
  }) => {
    const sessionId = session.value?.sessionId ?? result.value?.sessionId;
    if (!sessionId || !result.value || leaveSubmitting.value) {
      return;
    }
    leaveSubmitting.value = true;
    try {
      await submitLeaveMessage(sessionId, result.value.visitorKey, value);
      leaveDone.value = true;
    } catch (e) {
      appendSystemError('chat.leave.failed', (e as Error).message);
    } finally {
      leaveSubmitting.value = false;
    }
  };

  /** 提交评价, 成功后展示感谢气泡并收起评价卡 */
  const handleRateSubmit = async (value: {
    score: number;
    tags: string[];
    remark: string;
  }) => {
    const sessionId = session.value?.sessionId;
    if (!sessionId || !result.value || rateSubmitting.value) {
      return;
    }
    rateSubmitting.value = true;
    try {
      await rateSession(sessionId, result.value.visitorKey, value);
      if (session.value) {
        session.value.rateScore = value.score;
      }
      appendSystemBubble(
        `rate_done_${sessionId}`,
        t('chat.rate.thanks')
      );
    } catch (e) {
      appendSystemError('chat.rate.failed', (e as Error).message);
    } finally {
      rateSubmitting.value = false;
    }
  };

  /** 暂不评价, 收起评价卡但保留「再次咨询」按钮 */
  const handleRateSkip = () => {
    rateSkipped.value = true;
  };

  /** 再次咨询: 重新接入建立新会话, 不清空已上屏的消息 */
  const handleReconsult = async () => {
    if (!result.value || reconsulting.value) {
      return;
    }
    reconsulting.value = true;
    try {
      const data = await bootstrap({
        channelCode,
        visitorKey: result.value.visitorKey,
        nickname: result.value.nickname,
        email: result.value.email,
        sourceUrl: window.location.href
      });
      // 直接改字段而不是替换整个 result, 避免触发 watch 重建连接并清空消息
      Object.assign(result.value, {
        sessionId: data.sessionId,
        sessionStatus: data.sessionStatus,
        aiMode: data.aiMode,
        agentName: data.agentName,
        welcomeText: data.welcomeText
      });
      session.value = await getSession(data.sessionId, data.visitorKey);
      await readSession(data.sessionId, data.visitorKey);
      // 新会话重置排队与转人工的本地状态
      handoffDone.value = false;
      rateSkipped.value = false;
      leaveForced.value = false;
      leaveDone.value = false;
      lastQueuePosition = 0;
      stopWaitingReply();
      appendWelcome(data.sessionId);
    } catch (e) {
      appendSystemError('chat.rate.reconsultFailed', (e as Error).message);
    } finally {
      reconsulting.value = false;
    }
  };

  /** 点击「转人工」: 确认后调接口, 成功后补一条系统气泡 */
  const handleHandoff = () => {
    const sessionId = session.value?.sessionId ?? result.value?.sessionId;
    if (!sessionId || !result.value || handoffLoading.value) {
      return;
    }
    ElMessageBox.confirm(t('chat.handoff.confirm'), t('chat.handoff.confirmTitle'), {
      type: 'warning'
    })
      .then(async () => {
        if (handoffLoading.value || handoffDone.value) {
          return;
        }
        handoffLoading.value = true;
        try {
          const data = await handoffSession(
            sessionId,
            result.value?.visitorKey ?? ''
          );
          handoffDone.value = true;
          stopWaitingReply();
          appendSystemBubble(
            `handoff_${data.sessionId}`,
            data.onlineAgent ? t('chat.handoff.queued') : t('chat.handoff.offline')
          );
          // 无人在线时直接展开留言卡片，避免访客无路可走
          leaveForced.value = !data.onlineAgent;
          await refreshSessionMeta(sessionId);
        } catch (e) {
          appendSystemError('chat.handoff.failed', (e as Error).message);
        } finally {
          handoffLoading.value = false;
        }
      })
      .catch(() => {});
  };

  /** 发送文本消息, 走 IM 由服务端 webhook 落库 */
  const handleSend = async (value: string) => {
    const sessionId = session.value?.sessionId ?? result.value?.sessionId;
    if (!sessionId || !result.value) {
      return;
    }
    sending.value = true;
    try {
      const clientMsgNo = createClientMsgNo(
        `visitor_${result.value.visitorId}`
      );
      const payload: ChatPayload = {
        version: 1,
        type: 'text',
        role: 'visitor',
        content: value,
        sessionId,
        meta: { clientMsgNo }
      };
      await imClient.sendText(payload);
      // 客户自己发的消息 IM 不会再回推一次，这里先上屏，靠 clientMsgNo 去重
      append(fromPayload(payload, clientMsgNo));
      content.value = '';
      startWaitingReply();
    } catch (e) {
      appendSystemError('chat.error.sendFailed', (e as Error).message);
    } finally {
      sending.value = false;
    }
  };

  /** 发送一条访客消息, 文本与附件共用 */
  const sendPayload = async (payload: ChatPayload) => {
    const clientMsgNo = String(payload.meta?.clientMsgNo ?? '');
    await imClient.sendText(payload);
    append(fromPayload(payload, clientMsgNo));
  };

  /** 点击快捷问题直接发送内容 */
  const handleQuickReply = async (item: QuickReplyItem) => {
    const sessionId = session.value?.sessionId ?? result.value?.sessionId;
    if (!sessionId || !result.value || !canSend.value) {
      return;
    }
    sending.value = true;
    try {
      const clientMsgNo = createClientMsgNo(
        `visitor_${result.value.visitorId}`
      );
      await sendPayload({
        version: 1,
        type: 'text',
        role: 'visitor',
        content: item.content,
        sessionId,
        meta: { clientMsgNo }
      });
      // 点过一次就把胶囊收起来, 与 Crisp 的快捷问题一致
      quickReplies.value = [];
      startWaitingReply();
    } catch (e) {
      appendSystemError('chat.error.sendFailed', (e as Error).message);
    } finally {
      sending.value = false;
    }
  };

  /** 上传附件并发送, 上传中先上一个占位气泡 */
  const handleUpload = async (file: File) => {
    const sessionId = session.value?.sessionId ?? result.value?.sessionId;
    if (!sessionId || !result.value || !canSend.value) {
      return;
    }
    const placeholderKey = `uploading_${Date.now()}`;
    const contentType = isImageFile(file) ? 'image' : 'file';
    replace(placeholderKey, {
      key: placeholderKey,
      content: file.name,
      header: visitorName.value,
      placement: 'end',
      status: 'uploading',
      fromRole: 'visitor',
      contentType,
      meta: { name: file.name, size: file.size, contentType: file.type },
      sessionId
    });

    uploading.value = true;
    let uploaded: UploadedFile;
    try {
      uploaded = await uploadVisitorFile(file);
    } catch (e) {
      replace(placeholderKey, {
        key: placeholderKey,
        content: t('chat.file.uploadFailed', {
          reason: (e as Error).message || t('chat.error.retry')
        }),
        header: t('chat.message.system'),
        placement: 'end',
        status: 'error',
        fromRole: 'system',
        sessionId
      });
      uploading.value = false;
      return;
    }
    uploading.value = false;

    const clientMsgNo = createClientMsgNo(`visitor_${result.value.visitorId}`);
    const payload: ChatPayload = {
      version: 1,
      type: contentType,
      role: 'visitor',
      content: uploaded.url,
      sessionId,
      meta: {
        clientMsgNo,
        fileId: uploaded.fileId,
        name: uploaded.name,
        size: uploaded.size,
        contentType: uploaded.contentType
      }
    };
    try {
      await sendPayload(payload);
      replace(placeholderKey, fromPayload(payload, clientMsgNo));
      startWaitingReply();
    } catch (e) {
      replace(placeholderKey, {
        key: placeholderKey,
        content: t('chat.error.sendFailed', {
          reason: (e as Error).message || t('chat.error.retry')
        }),
        header: t('chat.message.system'),
        placement: 'end',
        status: 'error',
        fromRole: 'system',
        sessionId
      });
    }
  };

  /** 按文件类型判断是否按图片发送 */
  const isImageFile = (file: File) => {
    if (file.type.startsWith('image/')) {
      return true;
    }
    return /\.(png|jpe?g|gif|webp|bmp)$/i.test(file.name);
  };

  /** 连接 IM 并拉取历史消息 */
  const initChat = async () => {
    const data = result.value;
    if (!data) {
      return;
    }
    reset();
    stream.reset();
    await refreshSession(data.sessionId);
    appendWelcome(data.sessionId);
    await imClient.connect(
      { uid: data.uid, token: data.token, wsUrl: data.wsUrl },
      {
        onMessage: handleImMessage,
        onEvent: handleImEvent,
        onReconnect: () => {
          // 断线重连后重新拉取历史, 恢复流式消息的终态
          refreshSession(data.sessionId);
        }
      }
    );
  };

  /** 接入成功后建立 IM 连接 */
  watch(
    result,
    (data) => {
      if (data) {
        initChat().catch(() => {
          // 连接失败时仍可通过重试重新接入
        });
      }
    },
    { immediate: true }
  );

  /** 拉取渠道可用的快捷问题, 失败时整块不渲染, 不影响聊天 */
  watch(
    config,
    (data) => {
      if (!data?.channelCode) {
        return;
      }
      listQuickReplies(data.channelCode)
        .then((list) => {
          quickReplies.value = list ?? [];
        })
        .catch(() => {
          quickReplies.value = [];
        });
    },
    { immediate: true }
  );

  /** 页面隐藏时销毁连接, 避免后台占用网关 */
  const handlePageHide = () => {
    imClient.destroy();
  };

  /** 面板获得可见性时清零未读, 重新进入页面时恢复排队轮询 */
  const handleVisibility = () => {
    if (document.visibilityState === 'visible') {
      clearUnread();
      if (session.value?.status === STATUS_QUEUEING) {
        pollQueue();
        syncQueuePolling();
      }
      return;
    }
    // 页面切到后台时不再请求排队信息
    stopQueuePolling();
  };

  /** 收起面板, 交给悬浮脚本处理 */
  const handleClose = () => {
    postToParent('kefu:close');
  };

  /** 面板切换语言后通知悬浮脚本, 让气泡文案与欢迎语同步 */
  const handleLocaleChange = (value: string) => {
    postToParent('kefu:lang', { locale: value });
  };

  /** 语言变化后同步气泡上的发送者名称, 面板内部切换与父窗口下发都会走到这里 */
  watch(currentLocale, () => {
    visitorName.value = t('chat.message.self');
    refreshHeader();
  });

  /** 接收父窗口消息 */
  const handleParentMessage = (event: MessageEvent) => {
    if (!embed || !parentOrigin || event.origin !== parentOrigin) {
      return;
    }
    const data = (event.data ?? {}) as {
      type?: string;
      data?: VisitorProfile & { locale?: string };
    };
    if (data.type === 'kefu:open') {
      panelVisible.value = true;
      clearUnread();
    }
    if (data.type === 'kefu:close') {
      panelVisible.value = false;
    }
    // 悬浮脚本切换语言时同步面板, 面板不刷新页面也不重建 IM 连接
    if (data.type === 'kefu:lang' && data.data?.locale) {
      setLocale(data.data.locale);
    }
    if (data.type === 'kefu:identify' && data.data) {
      identify({ nickname: data.data.nickname, email: data.data.email }).catch(
        () => {
          // 识别失败不影响已有会话
        }
      );
    }
  };

  window.addEventListener('pagehide', handlePageHide);
  document.addEventListener('visibilitychange', handleVisibility);
  window.addEventListener('message', handleParentMessage);

  // 面板加载完成后告知父窗口，父窗口据此同步语言与访客资料
  postToParent('kefu:ready', { locale: currentLocale.value });

  onBeforeUnmount(() => {
    stopWaitingReply();
    stopQueuePolling();
    clearAgentTyping();
    window.removeEventListener('pagehide', handlePageHide);
    document.removeEventListener('visibilitychange', handleVisibility);
    window.removeEventListener('message', handleParentMessage);
    imClient.destroy();
  });

  /** 供模板使用的类型 */
  export type { ChatMessageItem };
</script>

<style lang="scss" scoped>
  .chat-window {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--el-bg-color);
    overflow: hidden;
  }

  :global(html.is-embed) .chat-window {
    border: none;
    border-radius: 0;
    box-shadow: none;
  }

  .chat-window-body {
    display: flex;
    flex: 1 1 60px;
    flex-direction: column;
    min-height: 0;
    background: var(--el-fill-color-lighter);
  }

  .chat-window-messages {
    flex: 1 1 60px;
    min-height: 0;
    padding: 12px;
    overflow-y: auto;
  }

  /* 对话 / 帮助页签, 仅在悬浮脚本开启 data-tabs=help 时出现 */
  .chat-window-tabs {
    display: flex;
    justify-content: center;
    padding: 8px 12px 0;
  }

  .chat-window-help {
    flex: 1 1 60px;
    min-height: 0;
    padding: 12px;
    overflow-y: auto;
  }

  .chat-window-help-item {
    padding: 10px 12px;
    cursor: pointer;
    background: var(--el-bg-color);
    border-radius: 6px;

    & + & {
      margin-top: 8px;
    }

    &:hover {
      background: var(--el-fill-color-light);
    }
  }

  .chat-window-help-title {
    font-weight: 500;
  }

  .chat-window-help-content {
    margin-top: 2px;
    font-size: 12px;
    line-height: 18px;
    color: var(--el-text-color-secondary);
  }

  .chat-window-message {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    margin-bottom: 14px;
  }

  .chat-window-message.is-visitor {
    align-items: flex-end;
  }

  .chat-window-message-header {
    margin-bottom: 4px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .chat-window-message .chat-bubble {
    max-width: 78%;
    padding: 8px 12px;
    font-size: 14px;
    border-radius: 6px;
    background: var(--el-bg-color);
  }

  .chat-window-message.is-visitor .chat-bubble {
    background: var(--el-color-primary-light-9);
  }

  /* 转人工后在最后一条 AI 气泡下方补一句灰字 */
  .chat-window-handoff-hint {
    margin-top: 2px;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
  }

  /* 对方正在输入的提示 */
  .chat-window-typing {
    display: flex;
    align-items: center;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    gap: 6px;
  }

  .chat-window-typing-dots {
    display: inline-flex;
    align-items: center;
    gap: 3px;

    i {
      display: block;
      width: 5px;
      height: 5px;
      background: var(--el-text-color-secondary);
      border-radius: 50%;
      animation: chat-typing-bounce 1.2s infinite ease-in-out;

      &:nth-child(2) {
        animation-delay: 0.15s;
      }

      &:nth-child(3) {
        animation-delay: 0.3s;
      }
    }
  }

  @keyframes chat-typing-bounce {
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

  /* 会话结束后常驻在输入区下方的「再次咨询」入口 */
  .chat-window-reconsult {
    display: flex;
    justify-content: center;
    padding: 10px 12px;
    background: var(--el-bg-color);
  }
</style>
