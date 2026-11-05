import { WKIM, WKIMChannelType, WKIMEvent } from 'easyjssdk';
import type { EventNotification, RecvMessage } from 'easyjssdk';
import { t } from '@/i18n';
import type { ChatPayload } from '@/api/types';

/** 连接 IM 需要的凭据 */
export interface ImCredential {
  /** 用户标识 */
  uid: string;
  /** 用户 token */
  token: string;
  /** gateway 地址 */
  wsUrl: string;
}

/** IM 事件回调 */
export interface ImHandlers {
  /** 收到消息 */
  onMessage: (message: RecvMessage) => void;
  /** 收到自定义事件, AI 流式增量走这里 */
  onEvent: (event: EventNotification) => void;
  /** 重连成功 */
  onReconnect?: () => void;
  /** 连接异常 */
  onError?: (error: unknown) => void;
}

/** 返回会话对应的群频道标识, 与服务端保持一致 */
export function sessionChannelId(sessionId: number) {
  return `session_${sessionId}`;
}

/** 生成 IM 侧的幂等消息编号 */
export function createClientMsgNo(prefix: string) {
  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`;
}

/** 聊天窗的 IM 连接封装, 只负责连接、发送与销毁 */
export class ChatImClient {
  /** SDK 实例 */
  private im: WKIM | null = null;

  /** 事件回调 */
  private handlers: ImHandlers | null = null;

  /**
   * 建立连接并注册事件
   * @param credential 连接凭据
   * @param handlers 事件回调
   */
  async connect(credential: ImCredential, handlers: ImHandlers) {
    this.destroy();
    if (!credential.wsUrl) {
      throw new Error(t('chat.error.imUrlMissing'));
    }
    this.handlers = handlers;
    const im = WKIM.init(credential.wsUrl, {
      uid: credential.uid,
      token: credential.token
    });
    im.on(WKIMEvent.Message, (message) => {
      this.handlers?.onMessage(message as RecvMessage);
    });
    im.on(WKIMEvent.CustomEvent, (event) => {
      this.handlers?.onEvent(event as EventNotification);
    });
    im.on(WKIMEvent.Reconnecting, () => {
      this.handlers?.onReconnect?.();
    });
    im.on(WKIMEvent.Error, (error) => {
      this.handlers?.onError?.(error);
    });
    await im.connect();
    this.im = im;
  }

  /**
   * 发送文本消息, 服务端通过 webhook 落库
   * @param payload 消息业务协议
   */
  async sendText(payload: ChatPayload) {
    if (!this.im) {
      throw new Error(t('chat.error.imNotConnected'));
    }
    return this.im.send(
      sessionChannelId(payload.sessionId),
      WKIMChannelType.Group,
      payload as unknown as object,
      { clientMsgNo: payload.meta?.clientMsgNo as string | undefined, header: { redDot: true } }
    );
  }

  /** 销毁连接, 页面卸载时调用 */
  destroy() {
    this.im?.destroy();
    this.im = null;
    this.handlers = null;
  }
}
