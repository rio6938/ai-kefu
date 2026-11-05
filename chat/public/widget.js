/**
 * AI 智能客服悬浮组件（类 Crisp 用法）
 *
 * 宿主页面一行引入即可在右下角出现客服气泡：
 * <script src="https://www.example.com/widget.js" data-channel="web-default"></script>
 *
 * 支持的数据属性：
 * data-channel  渠道标识，默认 web-default
 * data-lang     语言，zh-CN / en，默认按 KEFU_WIDGET_CONFIG.locale → html lang → 浏览器语言
 * data-position 位置，right / left，默认 right
 * data-color    主色，默认取渠道 themeColor，取不到时用 #409eff
 * data-welcome  欢迎气泡开关，0 表示不展示，默认展示一次
 * data-hidden   1 表示初始隐藏，之后可调用 KeFuWidget.setVisible(true) 显示
 * data-invite   主动邀请，格式「延迟秒,间隔秒,次数」，如 5,60,3；为空表示不邀请
 * data-autopopup 1 表示访问期间自动展开一次面板
 * data-draggable 1 表示气泡可拖动，位置记在 localStorage
 * data-theme    dark / auto / light，dark 固定深色，auto 跟随系统，默认浅色
 * data-tabs     面板内页签，chat 对话、help 帮助，逗号分隔，默认只有 chat
 * data-browse   1 表示上报浏览轨迹（还需后台开启 kefu.browse.enabled）
 *
 * 对外 API：window.KeFuWidget.open/close/toggle/setLocale/setVisible/setUnread/identify
 * 该脚本为零依赖原生实现，全部样式挂在 #kefu-widget-root 下，不影响宿主样式。
 */
(function () {
  'use strict';

  // 幂等：重复引入只渲染一次
  if (window.__kefuWidgetLoaded) {
    return;
  }
  if (document.getElementById('kefu-widget-root')) {
    window.__kefuWidgetLoaded = true;
    return;
  }
  window.__kefuWidgetLoaded = true;

  /** 允许的语言代码白名单，未知语言回退简体中文；zh-TW 为预留语言 */
  var LOCALES = ['zh-CN', 'en', 'zh-TW'];

  /** 默认语言 */
  var DEFAULT_LOCALE = 'zh-CN';

  /**
   * 脚本层文案表，键名与面板的 widget.* 前缀保持一致
   * 面板未加载完或加载失败时，气泡与欢迎语也用这份表，保证语言正确
   */
  var MESSAGES = {
    'zh-CN': {
      'widget.aria.open': '打开在线客服',
      'widget.aria.close': '收起在线客服',
      'widget.teaser.title': '在线客服',
      'widget.teaser.description': '有任何问题随时点这里咨询',
      'widget.teaser.agent': '客服小助手',
      'widget.greeting.morning': '早上好',
      'widget.greeting.afternoon': '下午好',
      'widget.greeting.evening': '晚上好',
      'widget.greeting.night': '夜深了',
      'widget.lang.name': '简体中文',
      'widget.invite.title': '需要帮助吗？',
      'widget.invite.desc': '有问题随时问我，人工客服也在线',
      'widget.invite.accept': '咨询一下',
      'widget.invite.decline': '暂时不用'
    },
    en: {
      'widget.aria.open': 'Open live chat',
      'widget.aria.close': 'Close live chat',
      'widget.teaser.title': 'Live chat',
      'widget.teaser.description': 'Click here to talk to us',
      'widget.teaser.agent': 'Support team',
      'widget.greeting.morning': 'Good morning',
      'widget.greeting.afternoon': 'Good afternoon',
      'widget.greeting.evening': 'Good evening',
      'widget.greeting.night': 'Hi there',
      'widget.lang.name': 'English',
      'widget.invite.title': 'Need a hand?',
      'widget.invite.desc': 'Ask us anything, human agents are online too',
      'widget.invite.accept': 'Chat now',
      'widget.invite.decline': 'Not now'
    }
  };

  /** 欢迎气泡已读标记, 与访客资料同放在 localStorage */
  var TEASER_KEY = 'kefuWidgetTeaser';

  /** 访客资料键, 与聊天窗共用，便于 identify 后直接生效 */
  var PROFILE_KEY = 'kefuVisitorProfile';

  /** 未读消息上限, 超过后显示 99+ */
  var UNREAD_MAX = 99;

  /** 当前脚本标签, 读取 data-* 配置 */
  var script =
    document.currentScript ||
    (function () {
      var list = document.getElementsByTagName('script');
      return list.length ? list[list.length - 1] : null;
    })();

  /** 读取数据属性 */
  var readAttr = function (name) {
    if (!script || !script.getAttribute) {
      return '';
    }
    var value = script.getAttribute('data-' + name);
    return value == null ? '' : String(value).trim();
  };

  /** 归一化语言代码，未知语言回退简体中文 */
  var normalizeLocale = function (value) {
    var lang = String(value || '').trim();
    if (!lang) {
      return '';
    }
    for (var i = 0; i < LOCALES.length; i++) {
      if (LOCALES[i].toLowerCase() === lang.toLowerCase()) {
        return LOCALES[i];
      }
    }
    var lower = lang.toLowerCase();
    if (lower.indexOf('zh') === 0) {
      return lower.indexOf('tw') >= 0 || lower.indexOf('hk') >= 0
        ? 'zh-TW'
        : 'zh-CN';
    }
    if (lower.indexOf('en') === 0) {
      return 'en';
    }
    return '';
  };

  /** 按优先级解析当前语言 */
  var resolveLocale = function () {
    var config = window.KEFU_WIDGET_CONFIG || {};
    var list = [
      readAttr('lang'),
      config.locale,
      document.documentElement ? document.documentElement.lang : '',
      navigator.language || navigator.userLanguage || ''
    ];
    for (var i = 0; i < list.length; i++) {
      var locale = normalizeLocale(list[i]);
      if (locale) {
        return locale;
      }
    }
    return DEFAULT_LOCALE;
  };

  /** 取文案，缺失时回退简体中文 */
  var pickMessage = function (locale, key) {
    var messages = MESSAGES[locale] || MESSAGES[DEFAULT_LOCALE];
    if (messages && messages[key] != null) {
      return messages[key];
    }
    return MESSAGES[DEFAULT_LOCALE][key] || key;
  };

  /** 按当前语言取文案 */
  var t = function (key) {
    return pickMessage(locale, key);
  };

  /** 按本地时间取问候语 key，与面板的 chat.greeting.* 分段保持一致 */
  var greetingKey = function () {
    var hour = new Date().getHours();
    if (hour >= 5 && hour < 12) {
      return 'widget.greeting.morning';
    }
    if (hour >= 12 && hour < 18) {
      return 'widget.greeting.afternoon';
    }
    if (hour >= 18 && hour < 23) {
      return 'widget.greeting.evening';
    }
    return 'widget.greeting.night';
  };

  /** 读取 localStorage，异常时返回空串 */
  var readStorage = function (key) {
    try {
      return window.localStorage.getItem(key) || '';
    } catch (e) {
      return '';
    }
  };

  /** 写入 localStorage，异常时忽略（隐私模式） */
  var writeStorage = function (key, value) {
    try {
      window.localStorage.setItem(key, value);
    } catch (e) {
      // 宿主禁用存储时不影响组件运行
    }
  };

  /** 读取 sessionStorage，异常时返回空串 */
  var readSession = function (key) {
    try {
      return window.sessionStorage.getItem(key) || '';
    } catch (e) {
      return '';
    }
  };

  /** 写入 sessionStorage，异常时忽略 */
  var writeSession = function (key, value) {
    try {
      window.sessionStorage.setItem(key, value);
    } catch (e) {
      // 宿主禁用存储时不影响组件运行
    }
  };

  /** 当前语言 */
  var locale = resolveLocale();

  /** 渠道标识 */
  var channel = readAttr('channel') || 'web-default';

  /** 展示位置 */
  var position = readAttr('position') === 'left' ? 'left' : 'right';

  /** 主色，配置里没写时由渠道公开配置回填 */
  var color = readAttr('color');

  /** 是否展示欢迎气泡 */
  var welcomeEnabled = readAttr('welcome') !== '0';

  /** 初始是否隐藏 */
  var visible = readAttr('hidden') !== '1' && readAttr('hidden') !== 'true';

  /** 主动邀请配置: 延迟秒、间隔秒、次数 */
  var inviteConfig = (function () {
    var list = readAttr('invite')
      .split(',')
      .map(function (item) {
        return Number(String(item).trim());
      });
    if (!list.length || !list[0]) {
      return null;
    }
    return {
      delay: list[0] > 0 ? list[0] : 5,
      interval: list[1] > 0 ? list[1] : 60,
      times: list[2] > 0 ? list[2] : 1
    };
  })();

  /** 访问期间是否自动展开一次面板 */
  var autoPopup = readAttr('autopopup') === '1';

  /** 气泡是否可拖动 */
  var draggable = readAttr('draggable') === '1';

  /** 主题: dark 固定深色, auto 跟随系统, 其余为浅色 */
  var themeAttr = readAttr('theme').toLowerCase();
  var darkMode =
    themeAttr === 'dark' ||
    (themeAttr === 'auto' &&
      window.matchMedia &&
      window.matchMedia('(prefers-color-scheme: dark)').matches);

  /** 面板内的页签, 默认只有对话 */
  var panelTabs = (function () {
    var value = readAttr('tabs') || 'chat';
    var list = value
      .split(',')
      .map(function (item) {
        return String(item).trim();
      })
      .filter(function (item) {
        return item === 'chat' || item === 'help';
      });
    return list.length ? list : ['chat'];
  })();

  /** 是否上报浏览轨迹 */
  var browseEnabled = readAttr('browse') === '1';

  /** 访客标识的存储键, 与聊天窗共用 */
  var VISITOR_KEY = 'kefuVisitorKey';

  /** 邀请已拒绝的标记, 同一次访问不再邀请 */
  var INVITE_DECLINED_KEY = 'kefuWidgetInviteDeclined';

  /** 面板是否展开 */
  var opened = false;

  /** 未读数 */
  var unread = 0;

  /** 面板是否已发出 ready */
  var panelReady = false;

  /** 访客资料 */
  var profile = null;

  /** 面板地址, 来自 kefu.web.chat_url */
  var chatUrl = '';

  /** 面板域名, postMessage 的 targetOrigin */
  var chatOrigin = '';

  /** 根容器 */
  var root = document.createElement('div');
  root.id = 'kefu-widget-root';
  root.className = 'kefu-widget is-' + position;

  /** 欢迎气泡 */
  var teaser = document.createElement('div');
  teaser.className = 'kefu-widget-teaser';
  teaser.setAttribute('role', 'button');
  teaser.setAttribute('tabindex', '0');
  teaser.style.display = 'none';

  var teaserTitle = document.createElement('div');
  teaserTitle.className = 'kefu-widget-teaser-title';
  var teaserDesc = document.createElement('div');
  teaserDesc.className = 'kefu-widget-teaser-desc';
  var teaserAgent = document.createElement('div');
  teaserAgent.className = 'kefu-widget-teaser-agent';
  teaser.appendChild(teaserAgent);
  teaser.appendChild(teaserTitle);
  teaser.appendChild(teaserDesc);

  /** 主动邀请气泡 */
  var invite = document.createElement('div');
  invite.className = 'kefu-widget-invite';
  invite.style.display = 'none';

  var inviteTitle = document.createElement('div');
  inviteTitle.className = 'kefu-widget-invite-title';
  var inviteDesc = document.createElement('div');
  inviteDesc.className = 'kefu-widget-invite-desc';
  var inviteActions = document.createElement('div');
  inviteActions.className = 'kefu-widget-invite-actions';
  var inviteAccept = document.createElement('button');
  inviteAccept.type = 'button';
  inviteAccept.className = 'kefu-widget-invite-accept';
  var inviteDecline = document.createElement('button');
  inviteDecline.type = 'button';
  inviteDecline.className = 'kefu-widget-invite-decline';
  inviteActions.appendChild(inviteAccept);
  inviteActions.appendChild(inviteDecline);
  invite.appendChild(inviteTitle);
  invite.appendChild(inviteDesc);
  invite.appendChild(inviteActions);

  /** 聊天面板容器 */
  var panel = document.createElement('div');
  panel.className = 'kefu-widget-panel';

  /** 气泡按钮 */
  var launcher = document.createElement('button');
  launcher.className = 'kefu-widget-launcher';
  launcher.type = 'button';

  /** 气泡图标（对话样式） */
  var icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  icon.setAttribute('viewBox', '0 0 24 24');
  icon.setAttribute('aria-hidden', 'true');
  var iconPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  iconPath.setAttribute(
    'd',
    'M12 3C6.99 3 3 6.36 3 10.5c0 2.2 1.14 4.16 2.94 5.5L5 21l4.1-2.05c.92.24 1.9.38 2.9.38 5.01 0 9-3.36 9-7.83S17.01 3 12 3z'
  );
  iconPath.setAttribute('fill', 'currentColor');
  icon.appendChild(iconPath);

  /** 未读红点 */
  var badge = document.createElement('span');
  badge.className = 'kefu-widget-badge';
  badge.style.display = 'none';

  launcher.appendChild(icon);
  launcher.appendChild(badge);

  root.appendChild(teaser);
  root.appendChild(invite);
  root.appendChild(panel);
  root.appendChild(launcher);

  /** 脚本样式，全部挂在 #kefu-widget-root 下，不引入全局 reset */
  var css = [
    '#kefu-widget-root{position:fixed;bottom:24px;z-index:2147483000;display:flex;flex-direction:column;align-items:flex-end;gap:12px;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;}',
    '#kefu-widget-root.is-left{left:24px;align-items:flex-start;}',
    '#kefu-widget-root.is-right{right:24px;}',
    '#kefu-widget-root *{box-sizing:border-box;}',
    '#kefu-widget-root .kefu-widget-launcher{position:relative;display:flex;align-items:center;justify-content:center;width:56px;height:56px;padding:0;border:none;border-radius:50%;background:var(--kefu-widget-color,#409eff);color:#fff;cursor:pointer;box-shadow:0 6px 16px rgba(0,0,0,.18);transition:transform .2s ease,box-shadow .2s ease;}',
    '#kefu-widget-root .kefu-widget-launcher:hover{transform:scale(1.05);box-shadow:0 8px 20px rgba(0,0,0,.24);}',
    '#kefu-widget-root .kefu-widget-launcher:focus-visible{outline:2px solid var(--kefu-widget-color,#409eff);outline-offset:2px;}',
    '#kefu-widget-root .kefu-widget-launcher svg{width:26px;height:26px;}',
    '#kefu-widget-root .kefu-widget-badge{position:absolute;top:-2px;right:-2px;min-width:18px;height:18px;padding:0 4px;font-size:11px;font-weight:600;line-height:18px;text-align:center;color:#fff;background:#f56c6c;border-radius:9px;}',
    '#kefu-widget-root .kefu-widget-teaser{max-width:260px;padding:12px 14px;border-radius:12px;background:#fff;box-shadow:0 6px 20px rgba(0,0,0,.14);cursor:pointer;}',
    '#kefu-widget-root .kefu-widget-teaser:focus-visible{outline:2px solid var(--kefu-widget-color,#409eff);outline-offset:2px;}',
    '#kefu-widget-root .kefu-widget-teaser-agent{font-size:12px;color:#909399;}',
    '#kefu-widget-root .kefu-widget-teaser-title{margin-top:2px;font-size:15px;font-weight:600;color:#303133;}',
    '#kefu-widget-root .kefu-widget-teaser-desc{margin-top:4px;font-size:13px;line-height:1.5;color:#606266;}',
    '#kefu-widget-root .kefu-widget-panel{display:none;width:400px;height:620px;overflow:hidden;border-radius:12px;background:#fff;box-shadow:0 12px 32px rgba(0,0,0,.18);}',
    '#kefu-widget-root.is-open .kefu-widget-panel{display:block;}',
    '#kefu-widget-root.is-open .kefu-widget-teaser{display:none!important;}',
    '#kefu-widget-root .kefu-widget-panel iframe{display:block;width:100%;height:100%;border:none;background:#fff;}',
    '#kefu-widget-root .kefu-widget-invite{max-width:260px;padding:12px 14px;border-radius:12px;background:#fff;box-shadow:0 6px 20px rgba(0,0,0,.14);}',
    '#kefu-widget-root .kefu-widget-invite-title{font-size:15px;font-weight:600;color:#303133;}',
    '#kefu-widget-root .kefu-widget-invite-desc{margin-top:4px;font-size:13px;line-height:1.5;color:#606266;}',
    '#kefu-widget-root .kefu-widget-invite-actions{display:flex;justify-content:flex-end;margin-top:10px;gap:8px;}',
    '#kefu-widget-root .kefu-widget-invite-actions button{padding:4px 12px;font-size:13px;border-radius:6px;cursor:pointer;border:1px solid transparent;}',
    '#kefu-widget-root .kefu-widget-invite-accept{background:var(--kefu-widget-color,#409eff);color:#fff;}',
    '#kefu-widget-root .kefu-widget-invite-decline{background:transparent;color:#909399;border-color:#dcdfe6;}',
    '#kefu-widget-root .kefu-widget-launcher.is-dragging{cursor:grabbing;transition:none;}',
    '#kefu-widget-root.is-dark .kefu-widget-teaser,#kefu-widget-root.is-dark .kefu-widget-invite{background:#1f1f1f;box-shadow:0 6px 20px rgba(0,0,0,.5);}',
    '#kefu-widget-root.is-dark .kefu-widget-teaser-title,#kefu-widget-root.is-dark .kefu-widget-invite-title{color:#e5eaf3;}',
    '#kefu-widget-root.is-dark .kefu-widget-teaser-desc,#kefu-widget-root.is-dark .kefu-widget-invite-desc,#kefu-widget-root.is-dark .kefu-widget-teaser-agent{color:#a3a6ad;}',
    '#kefu-widget-root.is-dark .kefu-widget-invite-decline{color:#a3a6ad;border-color:#4c4d4f;}',
    '#kefu-widget-root.is-dark .kefu-widget-panel{background:#141414;}',
    '#kefu-widget-root.is-hidden{display:none;}',
    '@media (max-width:768px){#kefu-widget-root{bottom:12px;gap:8px;}#kefu-widget-root.is-left{left:12px;}#kefu-widget-root.is-right{right:12px;}#kefu-widget-root.is-open{bottom:0;left:0;right:0;}#kefu-widget-root .kefu-widget-panel{width:100vw;height:100vh;border-radius:0;}#kefu-widget-root.is-open .kefu-widget-launcher{display:none;}}'
  ].join('');

  /** 注入样式，同一页面只注入一次 */
  var injectStyle = function () {
    if (document.getElementById('kefu-widget-style')) {
      return;
    }
    var style = document.createElement('style');
    style.id = 'kefu-widget-style';
    style.appendChild(document.createTextNode(css));
    (document.head || document.documentElement).appendChild(style);
  };

  /** 渲染随语言变化与未读变化的文案 */
  var render = function () {
    launcher.setAttribute(
      'aria-label',
      t(opened ? 'widget.aria.close' : 'widget.aria.open')
    );
    // 欢迎气泡标题用时间段问候语，与面板空消息时的问候保持一致
    teaserTitle.textContent = t(greetingKey());
    teaserDesc.textContent = t('widget.teaser.description');
    teaserAgent.textContent = t('widget.teaser.agent');
    // 面板 iframe 的无障碍标题也跟随语言
    var frame = panel.querySelector('iframe');
    if (frame) {
      frame.setAttribute('title', t('widget.teaser.title'));
    }
    inviteTitle.textContent = t('widget.invite.title');
    inviteDesc.textContent = t('widget.invite.desc');
    inviteAccept.textContent = t('widget.invite.accept');
    inviteDecline.textContent = t('widget.invite.decline');
    if (darkMode) {
      root.classList.add('is-dark');
    } else {
      root.classList.remove('is-dark');
    }
    if (unread > 0) {
      badge.textContent =
        unread > UNREAD_MAX ? UNREAD_MAX + '+' : String(unread);
      badge.style.display = 'block';
    } else {
      badge.textContent = '';
      badge.style.display = 'none';
    }
    if (visible) {
      root.classList.remove('is-hidden');
    } else {
      root.classList.add('is-hidden');
    }
    if (
      welcomeEnabled &&
      !readStorage(TEASER_KEY) &&
      !opened &&
      visible &&
      invite.style.display !== 'block'
    ) {
      teaser.style.display = 'block';
    } else {
      teaser.style.display = 'none';
    }
  };

  /** 解析面板域名，用于限制 postMessage 的目标 */
  var resolveOrigin = function (url) {
    try {
      var parsed = new URL(url, window.location.href);
      return parsed.protocol + '//' + parsed.host;
    } catch (e) {
      return '';
    }
  };

  /** 拼接面板地址，带上渠道、嵌入模式与语言 */
  var buildPanelUrl = function () {
    if (!chatUrl) {
      return '';
    }
    var params = ['channel=' + encodeURIComponent(channel)];
    params.push('embed=widget');
    params.push('lang=' + encodeURIComponent(locale));
    params.push('theme=' + (darkMode ? 'dark' : 'light'));
    params.push('tabs=' + encodeURIComponent(panelTabs.join(',')));
    var hashIndex = chatUrl.indexOf('#');
    var hash = hashIndex >= 0 ? chatUrl.slice(hashIndex) : '';
    var base = hashIndex >= 0 ? chatUrl.slice(0, hashIndex) : chatUrl;
    // 地址本身已带 embed / lang 时先剔除，避免出现重复参数
    base = base
      .replace(/([?&])(embed|lang|theme|tabs)=[^&]*&?/g, '$1')
      .replace(/[?&]$/, '');
    var sep = base.indexOf('?') >= 0 ? '&' : '?';
    return base + sep + params.join('&') + hash;
  };

  /** 向面板发消息，targetOrigin 使用面板域名 */
  var postToPanel = function (type, payload) {
    var frame = panel.querySelector('iframe');
    if (!frame || !frame.contentWindow || !chatOrigin) {
      return;
    }
    frame.contentWindow.postMessage(
      { type: type, data: payload || {} },
      chatOrigin
    );
  };

  /** 创建面板 iframe，只创建一次；地址缺失时返回 false */
  var ensureFrame = function () {
    if (panel.querySelector('iframe')) {
      return true;
    }
    var url = buildPanelUrl();
    if (!url) {
      return false;
    }
    var frame = document.createElement('iframe');
    frame.setAttribute('title', t('widget.teaser.title'));
    frame.setAttribute('allow', 'clipboard-write');
    frame.src = url;
    panel.appendChild(frame);
    return true;
  };

  /** 展开面板 */
  var open = function () {
    // 面板地址未配置（kefu.web.chat_url 为空）时不展开，避免出现空白框
    if (!ensureFrame()) {
      return;
    }
    hideInvite();
    visible = true;
    opened = true;
    root.classList.add('is-open');
    setUnread(0);
    writeStorage(TEASER_KEY, '1');
    render();
    postToPanel('kefu:open', {});
  };

  /** 收起面板 */
  var close = function () {
    opened = false;
    // 已看到过面板，红点清零；收起后新到的消息会重新计数
    unread = 0;
    root.classList.remove('is-open');
    render();
    // 告知面板已收起，期间到达的新消息计入未读
    postToPanel('kefu:close', {});
  };

  /** 切换开合 */
  var toggle = function () {
    if (opened) {
      close();
    } else {
      open();
    }
  };

  /** 设置语言，同时同步给面板；语言未变化时不重复通知，避免来回打转 */
  var setLocale = function (lang) {
    var next = normalizeLocale(lang);
    if (!next || next === locale) {
      return;
    }
    locale = next;
    render();
    var frame = panel.querySelector('iframe');
    if (frame) {
      // 面板未加载完时改 src，保证首次加载就是目标语言
      if (!panelReady) {
        var url = buildPanelUrl();
        if (url && frame.src !== url) {
          frame.src = url;
        }
      } else {
        postToPanel('kefu:lang', { locale: locale });
      }
    }
  };

  /** 设置显示/隐藏 */
  var setVisible = function (value) {
    visible = !!value;
    if (!visible) {
      close();
    }
    render();
  };

  /** 设置未读数 */
  var setUnread = function (value) {
    var count = Number(value);
    unread = !count || count < 0 ? 0 : Math.floor(count);
    render();
  };

  /** 识别访客，只接受昵称与邮箱 */
  var identify = function (data) {
    var source = data || {};
    var next = {};
    if (source.nickname) {
      next.nickname = String(source.nickname).trim();
    }
    if (source.email) {
      next.email = String(source.email).trim();
    }
    profile = next;
    try {
      writeStorage(PROFILE_KEY, JSON.stringify(next));
    } catch (e) {
      // 忽略存储异常
    }
    postToPanel('kefu:identify', next);
  };

  /** 接收面板消息 */
  var handleMessage = function (event) {
    if (!chatOrigin || event.origin !== chatOrigin) {
      return;
    }
    var data = event.data || {};
    switch (data.type) {
      case 'kefu:ready':
        panelReady = true;
        postToPanel('kefu:lang', { locale: locale });
        if (profile) {
          postToPanel('kefu:identify', profile);
        }
        break;
      case 'kefu:unread':
        setUnread(data.data ? data.data.count : 0);
        break;
      case 'kefu:close':
        close();
        break;
      case 'kefu:lang':
        setLocale(data.data ? data.data.locale : '');
        break;
      default:
        break;
    }
  };

  /** 读取渠道公开配置，回填主题色与面板地址 */
  var loadChannelConfig = function () {
    var config = window.KEFU_WIDGET_CONFIG || {};
    // 面板地址：优先宿主内联配置，其次脚本属性，最后取渠道公开配置下发的地址
    var chatBase = String(config.chatUrl || readAttr('chat-url') || '').trim();
    if (!chatBase && script && script.src) {
      // 未配置时按脚本自身所在目录推导，脚本与面板同源部署时无需额外配置
      try {
        chatBase = new URL('.', script.src).href;
      } catch (e) {
        chatBase = '';
      }
    }
    if (chatBase) {
      chatUrl = chatBase;
      chatOrigin = resolveOrigin(chatUrl);
    }
    // 接口地址：与面板同源时可以留空，跨域时用 data-api / apiBase 指定
    var api = String(config.apiBase || readAttr('api') || '')
      .trim()
      .replace(/\/$/, '');
    var req = new XMLHttpRequest();
    req.open(
      'GET',
      api + '/kefu/channel/' + encodeURIComponent(channel) + '/config',
      true
    );
    req.onload = function () {
      if (req.status < 200 || req.status >= 300) {
        return;
      }
      var body;
      try {
        body = JSON.parse(req.responseText);
      } catch (e) {
        return;
      }
      var config = body && body.code === 0 ? body.data : null;
      if (!config) {
        return;
      }
      if (!color && config.themeColor) {
        color = config.themeColor;
        root.style.setProperty('--kefu-widget-color', color);
      }
      if (!chatUrl && config.chatUrl) {
        chatUrl = config.chatUrl;
        chatOrigin = resolveOrigin(chatUrl);
      }
    };
    req.onerror = function () {
      // 配置拉取失败时组件仍可用，只是没有主题色
    };
    req.send();
  };

  /** 绑定事件 */
  var bindEvents = function () {
    launcher.addEventListener('click', toggle);
    teaser.addEventListener('click', open);
    teaser.addEventListener('keydown', function (event) {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        open();
      }
    });
    inviteAccept.addEventListener('click', function () {
      hideInvite();
      open();
    });
    inviteDecline.addEventListener('click', function () {
      hideInvite();
      writeSession(INVITE_DECLINED_KEY, '1');
      if (inviteTimer) {
        window.clearTimeout(inviteTimer);
        inviteTimer = null;
      }
    });
    document.addEventListener('keydown', function (event) {
      if (opened && event.key === 'Escape') {
        close();
      }
    });
    window.addEventListener('message', handleMessage);
  };

  /** 邀请次数计数 */
  var inviteShown = 0;

  /** 邀请定时器 */
  var inviteTimer = null;

  /** 展示邀请气泡，已拒绝或面板已展开时不展示 */
  var showInvite = function () {
    if (!inviteConfig || opened || !visible || readSession(INVITE_DECLINED_KEY)) {
      return;
    }
    inviteShown += 1;
    invite.style.display = 'block';
    teaser.style.display = 'none';
  };

  /** 隐藏邀请气泡 */
  var hideInvite = function () {
    invite.style.display = 'none';
  };

  /** 按配置调度邀请: 首次延迟展示，之后按间隔循环，达到次数后停止 */
  var scheduleInvite = function () {
    if (!inviteConfig || readSession(INVITE_DECLINED_KEY)) {
      return;
    }
    var tick = function () {
      if (opened || readSession(INVITE_DECLINED_KEY)) {
        return;
      }
      showInvite();
      if (inviteShown < inviteConfig.times) {
        inviteTimer = window.setTimeout(tick, inviteConfig.interval * 1000);
      }
    };
    inviteTimer = window.setTimeout(tick, inviteConfig.delay * 1000);
  };

  /** 访问期间自动展开一次面板 */
  var scheduleAutoPopup = function (delay) {
    if (!autoPopup || readStorage(TEASER_KEY + 'Auto')) {
      return;
    }
    window.setTimeout(function () {
      if (!opened) {
        writeStorage(TEASER_KEY + 'Auto', '1');
        open();
      }
    }, (delay || 3) * 1000);
  };

  /** 上报一次浏览轨迹，只采集页面标题与地址 */
  var reportBrowse = function () {
    if (!browseEnabled) {
      return;
    }
    var visitorKey = readStorage(VISITOR_KEY);
    if (!visitorKey) {
      return;
    }
    var config = window.KEFU_WIDGET_CONFIG || {};
    var api = String(config.apiBase || readAttr('api') || '')
      .trim()
      .replace(/\/$/, '');
    try {
      var req = new XMLHttpRequest();
      req.open('POST', api + '/kefu/visitor/browse', true);
      req.setRequestHeader('Content-Type', 'application/json');
      req.send(
        JSON.stringify({
          visitorKey: visitorKey,
          title: document.title || '',
          url: window.location.href
        })
      );
    } catch (e) {
      // 上报失败不影响组件
    }
  };

  /** 监听单页路由变化, 地址变化时补报一次浏览轨迹 */
  var watchBrowse = function () {
    if (!browseEnabled) {
      return;
    }
    reportBrowse();
    var lastUrl = window.location.href;
    window.setInterval(function () {
      if (window.location.href === lastUrl) {
        return;
      }
      lastUrl = window.location.href;
      reportBrowse();
    }, 3000);
    window.addEventListener('popstate', reportBrowse);
  };

  /** 拖动位置存储键 */
  var DRAG_KEY = 'kefuWidgetOffset';

  /** 让气泡可拖动，位置记在 localStorage */
  var bindDrag = function () {
    if (!draggable) {
      return;
    }
    var saved = readStorage(DRAG_KEY).split(',');
    var offsetX = Number(saved[0]) || 0;
    var offsetY = Number(saved[1]) || 0;
    var applyOffset = function () {
      root.style.transform =
        'translate(' + offsetX + 'px,' + offsetY + 'px)';
    };
    if (offsetX || offsetY) {
      applyOffset();
    }
    var dragging = false;
    var startX = 0;
    var startY = 0;
    var baseX = 0;
    var baseY = 0;
    launcher.addEventListener('mousedown', function (event) {
      dragging = true;
      startX = event.clientX;
      startY = event.clientY;
      baseX = offsetX;
      baseY = offsetY;
      launcher.classList.add('is-dragging');
      event.preventDefault();
    });
    document.addEventListener('mousemove', function (event) {
      if (!dragging) {
        return;
      }
      offsetX = baseX + event.clientX - startX;
      offsetY = baseY + event.clientY - startY;
      applyOffset();
    });
    document.addEventListener('mouseup', function () {
      if (!dragging) {
        return;
      }
      dragging = false;
      launcher.classList.remove('is-dragging');
      writeStorage(DRAG_KEY, offsetX + ',' + offsetY);
    });
    // 拖动过之后不触发点击展开
    launcher.addEventListener('click', function (event) {
      if (Math.abs(offsetX - baseX) > 3 || Math.abs(offsetY - baseY) > 3) {
        event.stopPropagation();
        event.preventDefault();
      }
    });
  };

  /** 挂载到宿主页面 */
  var mount = function () {
    if (color) {
      root.style.setProperty('--kefu-widget-color', color);
    }
    injectStyle();
    bindEvents();
    render();
    (document.body || document.documentElement).appendChild(root);
    loadChannelConfig();
    bindDrag();
    scheduleInvite();
    scheduleAutoPopup(inviteConfig ? inviteConfig.delay : 0);
    watchBrowse();
  };

  /** 对外 API，用法与 $crisp 对齐 */
  window.KeFuWidget = {
    /** 展开面板 */
    open: open,
    /** 收起面板 */
    close: close,
    /** 切换开合 */
    toggle: toggle,
    /** 切换语言 */
    setLocale: setLocale,
    /** 显示或隐藏整个组件 */
    setVisible: setVisible,
    /** 设置未读数 */
    setUnread: setUnread,
    /** 预填访客昵称与邮箱 */
    identify: identify
  };

  if (document.body) {
    mount();
  } else {
    document.addEventListener('DOMContentLoaded', mount);
  }
})();
