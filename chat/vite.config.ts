import { resolve } from 'node:path';
import { defineConfig, loadEnv } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd());
  // 开发环境把客户端接口与 OpenAI 兼容接口代理到后端
  const proxyTarget = env.VITE_API_PROXY_URL || 'http://127.0.0.1:8080';
  return {
    resolve: {
      alias: [{ find: /^@\//, replacement: resolve('src') + '/' }]
    },
    plugins: [vue()],
    server: {
      port: 5174,
      proxy: {
        '/kefu': { target: proxyTarget, changeOrigin: true },
        '/v1': { target: proxyTarget, changeOrigin: true }
      }
    }
  };
});
