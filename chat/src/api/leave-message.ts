import { post } from '@/utils/request';
import type { LeaveMessageForm } from './types';

/**
 * 提交离线留言
 * @param id 会话id
 * @param visitorKey 访客标识
 * @param data 留言内容
 */
export function submitLeaveMessage(
  id: number,
  visitorKey: string,
  data: LeaveMessageForm
) {
  return post<void>(
    `/kefu/session/${id}/leave-message?visitorKey=${encodeURIComponent(
      visitorKey
    )}`,
    data
  );
}
