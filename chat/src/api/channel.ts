import { get } from '@/utils/request';
import type { ChannelConfig } from './types';

/** 访客附件单文件上限的兜底值, 单位 MB, 后端未下发时使用 */
const DEFAULT_FILE_MAX_SIZE_MB = 10;

/** 访客附件允许后缀的兜底值, 不含点号, 后端未下发时使用 */
const DEFAULT_FILE_ALLOWED_EXT = [
  'jpg',
  'jpeg',
  'png',
  'gif',
  'webp',
  'pdf',
  'doc',
  'docx',
  'xls',
  'xlsx',
  'txt',
  'zip'
];

/**
 * 补齐渠道公开配置里的附件限额默认值:
 * 后端未下发时按默认限额兜底, 避免前端变成不限制大小与后缀
 * @param config 接口返回的渠道配置
 */
function withFileDefaults(config: ChannelConfig): ChannelConfig {
  return {
    ...config,
    fileMaxSizeMb: config.fileMaxSizeMb || DEFAULT_FILE_MAX_SIZE_MB,
    fileAllowedExt: config.fileAllowedExt?.length
      ? config.fileAllowedExt
      : DEFAULT_FILE_ALLOWED_EXT
  };
}

/**
 * 查询渠道的公开配置
 * @param code 渠道标识
 */
export function getChannelConfig(code: string) {
  return get<ChannelConfig>(
    `/kefu/channel/${encodeURIComponent(code)}/config`
  ).then(withFileDefaults);
}
