import { get } from '@/utils/request';
import type { QuickReplyItem } from './types';

/**
 * 查询渠道可用的访客快捷问题
 * @param channelCode 渠道标识
 */
export function listQuickReplies(channelCode: string) {
  return get<QuickReplyItem[]>('/kefu/quick-reply', { channelCode });
}
