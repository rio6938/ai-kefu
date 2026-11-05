import { t } from '@/i18n';
import type { ApiResult } from '@/api/types';

/** 接口地址前缀, 与后端同源反代时为空 */
const baseURL = (import.meta.env.VITE_API_URL ?? '').replace(/\/$/, '');

/**
 * 把查询参数拼接到地址上
 * @param path 接口路径
 * @param params 查询参数
 */
function withQuery(path: string, params?: Record<string, unknown>) {
  if (!params) {
    return `${baseURL}${path}`;
  }
  const search = new URLSearchParams();
  Object.keys(params).forEach((key) => {
    const value = params[key];
    if (value !== void 0 && value !== null && value !== '') {
      search.append(key, String(value));
    }
  });
  const query = search.toString();
  return query ? `${baseURL}${path}?${query}` : `${baseURL}${path}`;
}

/**
 * 解析统一响应, 业务码非 0 时抛出中文错误
 * @param response 响应
 */
async function parse<T>(response: Response): Promise<T> {
  let result: ApiResult<T>;
  try {
    result = (await response.json()) as ApiResult<T>;
  } catch {
    throw new Error(t('chat.error.badResponse'));
  }
  if (result.code === 0) {
    return result.data as T;
  }
  throw new Error(result.message || t('chat.error.requestFailed'));
}

/**
 * 发送 GET 请求
 * @param path 接口路径
 * @param params 查询参数
 */
export function get<T>(path: string, params?: Record<string, unknown>) {
  return fetch(withQuery(path, params)).then((res) => parse<T>(res));
}

/**
 * 发送 POST 请求
 * @param path 接口路径
 * @param body 请求体
 */
export function post<T>(path: string, body?: unknown) {
  return fetch(`${baseURL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body === void 0 ? void 0 : JSON.stringify(body)
  }).then((res) => parse<T>(res));
}
