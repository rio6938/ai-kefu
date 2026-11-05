/// <reference types="vite/client" />

/**
 * 环境变量
 */
interface ImportMetaEnv {
  /** 应用名称 */
  readonly VITE_APP_NAME?: string;
  /** 接口地址前缀 */
  readonly VITE_API_URL?: string;
  /** 开发环境接口代理地址 */
  readonly VITE_API_PROXY_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue';
  const component: DefineComponent<object, object, any>;
  export default component;
}
