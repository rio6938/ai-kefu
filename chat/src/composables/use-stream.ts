import type { Ref } from 'vue';
import type { EventNotification } from 'easyjssdk';
import { t } from '@/i18n';
import type { ChatMessageItem } from './use-messages';

/** 流式事件的数据体, 与服务端的 im.EventPayload 保持一致 */
interface StreamData {
  /** 本次增量文本 */
  delta?: string;
  /** 结束时的完整快照 */
  snapshot?: { text?: string };
  /** 中断或出错的原因 */
  reason?: string;
}

/**
 * AI 流式事件处理: 按 event_id 去重, 按 client_msg_no 归并到同一个气泡,
 * finish 之后把气泡置为终态
 * @param messages 消息列表
 */
export function useStream(messages: Ref<ChatMessageItem[]>) {
  /** 已处理的事件id, 用于重复投递去重 */
  const appliedEventIds = new Set<string>();

  /** 从事件里解析所属的消息编号, 事件id格式为 {clientMsgNo}:delta:{seq} */
  const resolveClientMsgNo = (event: EventNotification) => {
    const withNo = event as EventNotification & { clientMsgNo?: string };
    if (withNo.clientMsgNo) {
      return withNo.clientMsgNo;
    }
    const index = event.id ? event.id.indexOf(':') : -1;
    return index > 0 ? event.id.slice(0, index) : event.id;
  };

  /** 解析可能为字符串的事件数据 */
  const parseData = (data: EventNotification['data']) => {
    if (data == null) {
      return null;
    }
    if (typeof data !== 'string') {
      return data as StreamData;
    }
    try {
      return JSON.parse(data) as StreamData;
    } catch {
      return null;
    }
  };

  /** 应用一个流式事件 */
  const apply = (event: EventNotification) => {
    if (!event?.id || appliedEventIds.has(event.id)) {
      return;
    }
    appliedEventIds.add(event.id);

    const data = parseData(event.data);
    const clientMsgNo = resolveClientMsgNo(event);
    /** 查找气泡，缺失时补建一个，避免丢掉 open 事件后增量无处落 */
    const ensureTarget = () => {
      const found = messages.value.find(
        (item) => item.clientMsgNo === clientMsgNo
      );
      if (found) {
        return found;
      }
      const created = {
        key: clientMsgNo,
        content: '',
        header: t('chat.message.ai'),
        placement: 'start' as const,
        status: 'loading',
        fromRole: 'ai',
        clientMsgNo
      };
      messages.value.push(created);
      return created;
    };

    switch (event.type) {
      case 'stream.open':
        ensureTarget();
        break;
      case 'stream.delta':
        if (data?.delta) {
          const target = ensureTarget();
          target.content = (target.content ?? '') + data.delta;
          target.status = 'receiving';
        }
        break;
      case 'stream.finish': {
        const target = ensureTarget();
        if (data?.snapshot?.text != null) {
          target.content = data.snapshot.text;
        }
        target.status = 'finish';
        break;
      }
      case 'stream.cancel':
      case 'stream.error': {
        const target = messages.value.find(
          (item) => item.clientMsgNo === clientMsgNo
        );
        if (target) {
          target.status = 'finish';
        }
        break;
      }
      default:
        break;
    }
  };

  /** 清空已处理的事件id, 切换会话或重连时调用 */
  const reset = () => {
    appliedEventIds.clear();
  };

  return { apply, reset };
}
