import { ref } from 'vue';
import type { Ref } from 'vue';
import { t } from '@/i18n';
import { listMessages } from '@/api/session';
import type { ChatPayload, Message } from '@/api/types';

/** 附件消息的附加信息, 与服务端约定的 payload.meta 一致 */
export interface ChatFileMeta {
  /** 文件记录id */
  fileId?: number;
  /** 文件名称 */
  name?: string;
  /** 文件大小, 单位字节 */
  size?: number;
  /** 文件类型 */
  contentType?: string;
}

/** 聊天气泡 */
export interface ChatMessageItem {
  /** 气泡唯一标识 */
  key: string;
  /** 消息内容, 附件消息存附件访问地址 */
  content: string;
  /** 气泡上方的发送者名称, 空串表示不展示 */
  header: string;
  /** 左右布局, 访客发言在右侧 */
  placement: 'start' | 'end';
  /** 消息状态, 流式回复为 receiving, 上传中为 uploading */
  status?: string;
  /** 发送者角色, visitor/agent/ai/system */
  fromRole?: string;
  /** 内容类型, text/image/file/system/ai_answer */
  contentType?: string;
  /** 附件消息的附加信息, 文本消息为空 */
  meta?: ChatFileMeta;
  /** IM 侧幂等消息编号 */
  clientMsgNo?: string;
  /** 会话id */
  sessionId?: number;
  /** 引用的原消息摘要, 引用回复时有值 */
  quoteContent?: string;
}

/** 服务端下发的消息 payload 里可能带的 meta 字段 */
interface PayloadMeta extends ChatFileMeta {
  /** IM 幂等消息编号 */
  clientMsgNo?: string;
  /** 引用的原消息, 客服引用回复时下发 */
  quote?: { messageId?: number; content?: string; role?: string };
}

/** 消息列表与发送状态 */
export function useMessages(visitorName: Ref<string>) {
  /** 消息列表 */
  const messages = ref<ChatMessageItem[]>([]);

  /** 历史消息加载中 */
  const loading = ref(false);

  /** 发送中 */
  const sending = ref(false);

  /** 消息发送者名称, 切换语言后调用 refreshHeader 重新渲染 */
  const roleName = (role?: string) => {
    switch (role) {
      case 'agent':
        return t('chat.message.agent');
      case 'ai':
        return t('chat.message.ai');
      case 'system':
        return t('chat.message.system');
      default:
        return visitorName.value;
    }
  };

  /** 从消息附加信息里取出附件相关字段, 非附件消息返回空对象 */
  const readMeta = (meta?: unknown): ChatFileMeta | undefined => {
    if (!meta || typeof meta !== 'object') {
      return void 0;
    }
    const value = meta as PayloadMeta;
    if (!value.fileId && !value.name) {
      return void 0;
    }
    return {
      fileId: value.fileId,
      name: value.name,
      size: value.size,
      contentType: value.contentType
    };
  };

  /** 把接口返回的消息转成气泡 */
  const fromMessage = (message: Message): ChatMessageItem => ({
    key: String(message.messageId),
    content: message.content,
    header: roleName(message.fromRole),
    placement: message.fromRole === 'visitor' ? 'end' : 'start',
    status: 'finish',
    fromRole: message.fromRole,
    contentType: message.contentType,
    meta: readMeta(parsePayload(message.payload)),
    clientMsgNo: message.clientMsgNo,
    sessionId: message.sessionId,
    quoteContent: parsePayload(message.payload)?.quote?.content
  });

  /** 把 IM 消息的业务协议转成气泡 */
  const fromPayload = (
    payload: ChatPayload,
    clientMsgNo?: string
  ): ChatMessageItem => ({
    key: clientMsgNo ?? `payload_${Date.now()}`,
    content: payload.content,
    header: roleName(payload.role),
    placement: payload.role === 'visitor' ? 'end' : 'start',
    status: 'finish',
    fromRole: payload.role,
    contentType: payload.type,
    meta: readMeta(payload.meta),
    clientMsgNo,
    sessionId: payload.sessionId,
    quoteContent: (payload.meta as PayloadMeta | undefined)?.quote?.content
  });

  /** 解析消息原始 payload, 附件信息存在 payload.meta 里 */
  const parsePayload = (payload?: string): PayloadMeta | undefined => {
    if (!payload) {
      return void 0;
    }
    try {
      const value = JSON.parse(payload) as { meta?: PayloadMeta };
      return value?.meta;
    } catch {
      return void 0;
    }
  };

  /** 追加一条消息, 按幂等编号与消息id去重 */
  const append = (item: ChatMessageItem) => {
    const exists = messages.value.some(
      (message) =>
        message.key === item.key ||
        (item.clientMsgNo != null && message.clientMsgNo === item.clientMsgNo)
    );
    if (!exists) {
      messages.value.push(item);
    }
    return !exists;
  };

  /** 替换指定气泡, 上传占位气泡完成后用它更新成正式气泡 */
  const replace = (key: string, item: ChatMessageItem) => {
    const index = messages.value.findIndex((message) => message.key === key);
    if (index >= 0) {
      messages.value.splice(index, 1, item);
      return;
    }
    messages.value.push(item);
  };

  /** 拉取历史消息, 用于重连与刷新后恢复终态 */
  const loadHistory = async (sessionId: number, visitorKey: string) => {
    loading.value = true;
    try {
      const result = await listMessages(sessionId, visitorKey, {
        page: 1,
        limit: 100
      });
      messages.value = (result.list ?? []).map(fromMessage);
    } finally {
      loading.value = false;
    }
  };

  /** 清空消息 */
  const reset = () => {
    messages.value = [];
  };

  /** 切换语言后重算气泡上方的发送者名称, 消息内容本身不翻译 */
  const refreshHeader = () => {
    messages.value.forEach((item) => {
      if (item.fromRole === 'visitor') {
        item.header = visitorName.value;
        return;
      }
      item.header = roleName(item.fromRole);
    });
  };

  return {
    messages,
    loading,
    sending,
    append,
    replace,
    loadHistory,
    reset,
    refreshHeader,
    fromPayload,
    fromMessage
  };
}
