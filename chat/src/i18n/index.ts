import { createI18n } from 'vue-i18n';
import zhCN from './lang/zh-CN';
import en from './lang/en';

/** 默认语言, 未知语言一律回退到它 */
export const DEFAULT_LOCALE = 'zh-CN';

/** 支持的语言代码白名单, zh-TW 预留, 暂无语言包时回退简体中文 */
export const LOCALES = ['zh-CN', 'en', 'zh-TW'];

/** 语言代码类型 */
export type ChatLocale = 'zh-CN' | 'en' | 'zh-TW';

/** 语言在 localStorage 里的键名, 与悬浮脚本共用 */
const LOCALE_KEY = 'kefuLang';

/**
 * 归一化语言代码, 未知语言返回空串
 * @param value 原始语言代码
 */
export function normalizeLocale(value?: string | null): ChatLocale | '' {
  const lang = String(value ?? '').trim();
  if (!lang) {
    return '';
  }
  const matched = LOCALES.find(
    (item) => item.toLowerCase() === lang.toLowerCase()
  );
  if (matched) {
    return matched as ChatLocale;
  }
  const lower = lang.toLowerCase();
  if (lower.startsWith('zh')) {
    return lower.includes('tw') || lower.includes('hk') ? 'zh-TW' : 'zh-CN';
  }
  if (lower.startsWith('en')) {
    return 'en';
  }
  return '';
}

/** 读取 localStorage 里的语言, 异常时返回空串 */
function readStoredLocale() {
  try {
    return localStorage.getItem(LOCALE_KEY) ?? '';
  } catch {
    return '';
  }
}

/**
 * 按优先级解析初始语言:
 * ?lang= > localStorage.kefuLang > html lang > 浏览器语言 > zh-CN
 */
export function resolveLocale(): ChatLocale {
  const candidates = [
    new URLSearchParams(window.location.search).get('lang'),
    readStoredLocale(),
    document.documentElement.lang,
    navigator.language ||
      (navigator as unknown as { userLanguage?: string }).userLanguage
  ];
  for (const item of candidates) {
    const locale = normalizeLocale(item);
    if (locale) {
      return locale;
    }
  }
  return DEFAULT_LOCALE;
}

/** 已提示过缺失的 key, 避免同一个 key 反复刷日志 */
const warnedKeys = new Set<string>();

/** 当前语言 */
const initialLocale = resolveLocale();

/** 语言包, 首批 zh-CN / en; zh-TW 为预留语言, 暂无译文时复用简体中文 */
const messages = {
  'zh-CN': zhCN,
  en,
  'zh-TW': zhCN
} as const;

/** i18n 实例, 面板与悬浮脚本各算各的语言, 这里只管面板 */
export const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: DEFAULT_LOCALE,
  messages,
  // 缺失 key 回退简体中文, 这里只在开发期提示一次, 便于排查漏翻
  missing: (_locale: string, key: string) => {
    if (!import.meta.env.DEV || warnedKeys.has(key)) {
      return;
    }
    warnedKeys.add(key);
    console.warn(`[i18n] missing key: ${key}`);
  }
});

/** 当前语言, 组件里用它渲染语言下拉与 Element Plus 语言包 */
export const locale = i18n.global.locale;

/**
 * 切换语言: 同步 i18n、html lang 与 localStorage
 * @param value 目标语言
 * @returns 切换后的语言, 语言不支持时返回当前语言
 */
export function setLocale(value?: string | null): ChatLocale {
  const next = normalizeLocale(value);
  if (!next) {
    return locale.value as ChatLocale;
  }
  locale.value = next;
  document.documentElement.lang = next;
  try {
    localStorage.setItem(LOCALE_KEY, next);
  } catch {
    // 宿主禁用存储时只影响记忆，不影响当前语言
  }
  return next;
}

/**
 * 组件外取文案, 供 api / composables / im 等非组件模块使用
 * @param key 文案 key
 * @param params 插值参数
 */
export function t(key: string, params?: Record<string, unknown>) {
  return params ? i18n.global.t(key, params) : i18n.global.t(key);
}

/** 语言下拉的数据源, 首批只开放有语言包的两种语言 */
export const localeOptions = [
  { value: 'zh-CN', labelKey: 'chat.lang.zhCN' },
  { value: 'en', labelKey: 'chat.lang.en' }
];

/** 按本地时间算出的问候语 key, 面板与脚本各自计算, 结果一致 */
export function greetingKey() {
  const hour = new Date().getHours();
  if (hour >= 5 && hour < 12) {
    return 'chat.greeting.morning';
  }
  if (hour >= 12 && hour < 18) {
    return 'chat.greeting.afternoon';
  }
  if (hour >= 18 && hour < 23) {
    return 'chat.greeting.evening';
  }
  return 'chat.greeting.night';
}

// html lang 与本地存储同步成解析结果, 保证脚本与面板读到的语言一致
document.documentElement.lang = initialLocale;
setLocale(initialLocale);
