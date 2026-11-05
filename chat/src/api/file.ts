import { t } from '@/i18n';
import type { ApiResult, UploadedFile } from './types';
/** 接口地址前缀, 与后端同源反代时为空 */
const baseURL = (import.meta.env.VITE_API_URL ?? '').replace(/\/$/, '');

/**
 * 上传访客附件, 走 multipart 与登录接口不同, 不能复用 request.ts 的 json 封装
 * @param file 文件
 */
export async function uploadVisitorFile(file: File): Promise<UploadedFile> {
  const formData = new FormData();
  formData.append('file', file, file.name);

  let response: Response;
  try {
    response = await fetch(`${baseURL}/kefu/file/upload`, {
      method: 'POST',
      body: formData
    });
  } catch {
    throw new Error(t('chat.error.uploadNetwork'));
  }

  let result: ApiResult<UploadedFile>;
  try {
    result = (await response.json()) as ApiResult<UploadedFile>;
  } catch {
    throw new Error(t('chat.error.badResponse'));
  }
  if (result.code === 0 && result.data) {
    return result.data;
  }
  throw new Error(result.message || t('chat.error.requestFailed'));
}
