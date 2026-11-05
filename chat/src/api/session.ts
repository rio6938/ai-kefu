import { get, post } from '@/utils/request';
import type {
  BootstrapForm,
  BootstrapResult,
  HandoffResult,
  Message,
  PageParam,
  PageResult,
  RateForm,
  Session
} from './types';

/**
 * 访客接入, 返回 IM 连接凭据与当前会话
 * @param data 接入参数
 */
export function bootstrap(data: BootstrapForm) {
  return post<BootstrapResult>('/kefu/bootstrap', data);
}

/**
 * 查询会话详情
 * @param id 会话id
 * @param visitorKey 访客标识
 */
export function getSession(id: number, visitorKey: string) {
  return get<Session>(`/kefu/session/${id}`, { visitorKey });
}

/**
 * 分页查询会话消息, 重连后用它恢复终态
 * @param id 会话id
 * @param visitorKey 访客标识
 * @param params 分页参数
 */
export function listMessages(
  id: number,
  visitorKey: string,
  params: PageParam
) {
  return get<PageResult<Message>>(`/kefu/session/${id}/messages`, {
    ...params,
    visitorKey
  });
}

/**
 * 清空会话未读
 * @param id 会话id
 * @param visitorKey 访客标识
 */
export function readSession(id: number, visitorKey: string) {
  return post<void>(`/kefu/session/${id}/read?visitorKey=${encodeURIComponent(visitorKey)}`);
}

/**
 * 访客主动转人工
 * @param id 会话id
 * @param visitorKey 访客标识
 */
export function handoffSession(id: number, visitorKey: string) {
  return post<HandoffResult>(
    `/kefu/session/${id}/handoff?visitorKey=${encodeURIComponent(visitorKey)}`
  );
}

/**
 * 上报访客正在输入
 * @param id 会话id
 * @param visitorKey 访客标识
 */
export function sendTyping(id: number, visitorKey: string) {
  return post<void>(
    `/kefu/session/${id}/typing?visitorKey=${encodeURIComponent(visitorKey)}`
  );
}

/**
 * 访客评价会话
 * @param id 会话id
 * @param visitorKey 访客标识
 * @param data 评价内容
 */
export function rateSession(
  id: number,
  visitorKey: string,
  data: RateForm
) {
  return post<void>(
    `/kefu/session/${id}/rate?visitorKey=${encodeURIComponent(visitorKey)}`,
    data
  );
}
