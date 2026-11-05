import { onMounted, ref } from 'vue';
import { t } from '@/i18n';
import { getChannelConfig } from '@/api/channel';
import { bootstrap } from '@/api/session';
import type {
  BootstrapResult,
  ChannelConfig,
  VisitorProfile
} from '@/api/types';

/** 访客标识在 localStorage 里的键名 */
const VISITOR_KEY = 'kefuVisitorKey';

/** 访客资料在 localStorage 里的键名 */
const PROFILE_KEY = 'kefuVisitorProfile';

/** 未指定渠道时使用的默认渠道 */
const DEFAULT_CHANNEL = 'web-default';

/** 读取本地保存的访客资料，格式异常时返回空对象 */
const readProfile = (): VisitorProfile => {
  try {
    const value = localStorage.getItem(PROFILE_KEY);
    if (!value) {
      return {};
    }
    const parsed = JSON.parse(value) as VisitorProfile;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
};

/** 保存访客资料，两个字段都为空时清除存储 */
const saveProfile = (profile: VisitorProfile) => {
  const nickname = (profile.nickname ?? '').trim();
  const email = (profile.email ?? '').trim();
  if (!nickname && !email) {
    localStorage.removeItem(PROFILE_KEY);
    return;
  }
  localStorage.setItem(PROFILE_KEY, JSON.stringify({ nickname, email }));
};

/** 访客接入: 读渠道参数 → 拉渠道配置 → bootstrap → 落 visitorKey */
export function useBootstrap() {
  /** 渠道标识 */
  const channelCode =
    new URLSearchParams(window.location.search).get('channel') ||
    DEFAULT_CHANNEL;

  /** 渠道公开配置 */
  const config = ref<ChannelConfig | null>(null);

  /** 接入结果 */
  const result = ref<BootstrapResult | null>(null);

  /** 接入中 */
  const loading = ref(true);

  /** 接入失败提示 */
  const error = ref('');

  /** 已保存的访客资料 */
  const profile = ref<VisitorProfile>(readProfile());

  /** 是否需要先填写访客资料, 首次进入且没有本地资料时为 true */
  const needProfile = ref(false);

  /** 执行接入 */
  const load = async () => {
    loading.value = true;
    error.value = '';
    try {
      config.value = await getChannelConfig(channelCode);
      const visitorKey = localStorage.getItem(VISITOR_KEY) ?? '';
      const data = await bootstrap({
        channelCode,
        visitorKey,
        nickname: profile.value.nickname,
        email: profile.value.email,
        sourceUrl: window.location.href
      });
      if (data.visitorKey) {
        localStorage.setItem(VISITOR_KEY, data.visitorKey);
      }
      result.value = data;
      profile.value = { nickname: data.nickname, email: data.email };
      // 接入成功即视为已过预填环节, 避免悬浮脚本的 identify 与预填卡片同时出现
      needProfile.value = false;
    } catch (e) {
      error.value = (e as Error).message || t('chat.session.connectFailed');
    } finally {
      loading.value = false;
    }
  };

  /** 首次进入且没有本地资料时先展示预填卡片，不拦截接入 */
  const prepare = async () => {
    const stored = localStorage.getItem(VISITOR_KEY);
    if (!stored && !readProfile().nickname && !readProfile().email) {
      needProfile.value = true;
      loading.value = false;
      return;
    }
    await load();
  };

  /** 提交预填资料后写入本地并重新接入 */
  const submitProfile = async (value: VisitorProfile) => {
    saveProfile(value);
    profile.value = readProfile();
    needProfile.value = false;
    await load();
  };

  /** 跳过预填, 只在未建立会话时生效 */
  const skipProfile = async () => {
    needProfile.value = false;
    await load();
  };

  /**
   * 修改资料：已建立会话时只补发 bootstrap 更新访客资料，
   * 不重建 IM 连接与消息列表；资料没变时不发请求
   */
  const updateProfile = async (value: VisitorProfile) => {
    const nickname = (value.nickname ?? '').trim();
    const email = (value.email ?? '').trim();
    const changed =
      nickname !== (profile.value.nickname ?? '') ||
      email !== (profile.value.email ?? '');
    saveProfile({ nickname, email });
    profile.value = readProfile();
    if (!changed) {
      return;
    }
    if (!result.value) {
      await load();
      return;
    }
    await bootstrap({
      channelCode,
      visitorKey: result.value.visitorKey,
      nickname,
      email,
      sourceUrl: window.location.href
    }).then((data) => {
      // 重新接入会重新登记 IM token，同步给当前会话，避免断线重连时用旧 token
      if (result.value && data.token) {
        result.value = { ...result.value, token: data.token };
      }
    });
  };

  /** 来自悬浮脚本的 identify, 只接受昵称与邮箱 */
  const identify = async (value: VisitorProfile) => {
    await updateProfile(value);
  };

  onMounted(prepare);

  return {
    channelCode,
    config,
    result,
    loading,
    error,
    profile,
    needProfile,
    reload: load,
    submitProfile,
    skipProfile,
    updateProfile,
    identify
  };
}
