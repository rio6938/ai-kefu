import { createApp } from 'vue';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';
import App from './App.vue';
import { i18n } from './i18n';
import './styles/index.scss';

const app = createApp(App);

/** 是否由悬浮组件以 iframe 方式嵌入, 嵌入时去掉整页背景与留白 */
if (new URLSearchParams(window.location.search).get('embed') === 'widget') {
  document.documentElement.classList.add('is-embed');
}

/** 深色主题由悬浮脚本通过 theme 参数下发, 与 Element Plus 的 dark 变量对齐 */
if (new URLSearchParams(window.location.search).get('theme') === 'dark') {
  document.documentElement.classList.add('dark');
}

// Element Plus 自带文案由 App.vue 里的 ElConfigProvider 按当前语言下发
app.use(i18n);
app.use(ElementPlus);
app.mount('#app');
