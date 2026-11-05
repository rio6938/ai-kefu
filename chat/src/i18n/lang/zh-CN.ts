/** 简体中文文案, 面板的全部用户可见文案都放这里 */
export default {
  chat: {
    lang: {
      zhCN: '简体中文',
      en: 'English'
    },
    greeting: {
      morning: '早上好',
      afternoon: '下午好',
      evening: '晚上好',
      night: '夜深了'
    },
    header: {
      editProfile: '修改资料',
      aiMode: 'AI 助手接待中',
      collapse: '收起',
      language: '语言',
      serving: '{name} 为您服务',
      title: '在线客服',
      status: {
        queue: '正在为您转接人工客服，请稍候',
        queueNext: '马上轮到您，请稍候',
        queuePosition: '您前面还有 {position} 人，预计等待约 {minutes} 分钟',
        ended: '会话已结束，可刷新重新咨询',
        leave: '客服暂不在线，请留言',
        human: '人工客服为您服务，通常几秒内回复',
        ai: 'AI 助手为您服务，可随时输入转人工'
      }
    },
    session: {
      connecting: '正在接入客服，请稍候',
      connectFailed: '接入客服失败',
      retry: '重试',
      empty: '{greeting}，有问题随时问我'
    },
    loading: {
      waiting: '客服正在回复，请稍候',
      waitingLong: '已收到您的消息，客服会尽快回复',
      ai: 'AI 正在生成回复，请稍候'
    },
    prechat: {
      title: '开始咨询前填写资料',
      editTitle: '修改资料',
      desc: '昵称与邮箱都可留空，填写后客服能更好地为您服务',
      nickname: '昵称',
      nicknamePlaceholder: '怎么称呼您',
      email: '邮箱',
      emailPlaceholder: '选填，用于接收回复',
      emailInvalid: '邮箱格式不正确',
      skip: '跳过',
      start: '开始咨询'
    },
    composer: {
      placeholder: '请输入想咨询的问题，回车发送',
      ended: '会话已结束',
      disabled: '暂不可发送消息',
      needLogin: '咨询前请先登录',
      aiTip: '内容由 AI 生成，仅供参考',
      send: '发送',
      attachment: '附件',
      attachmentTooltip: '发送附件',
      handoff: '转人工'
    },
    handoff: {
      confirmTitle: '系统提示',
      confirm: '确认转接人工客服？',
      queued: '已为您转接人工客服，正在排队',
      offline: '当前无人工客服在线，您可以留言，客服上线后联系您',
      failed: '转人工失败：{reason}',
      queueNext: '马上轮到您，请稍候',
      queueChanged: '您前面还有 {position} 人，预计等待约 {minutes} 分钟',
      transferred: '已转接人工客服'
    },
    rate: {
      title: '您对本次服务满意吗？',
      remarkPlaceholder: '还可以补充您的建议（选填）',
      submit: '提交评价',
      skip: '暂不评价',
      thanks: '感谢您的评价',
      again: '再次咨询',
      failed: '评价提交失败：{reason}',
      reconsultFailed: '再次咨询失败：{reason}',
      tags: {
        speed: '响应速度',
        professional: '专业程度',
        attitude: '服务态度',
        solved: '解决问题'
      }
    },
    leave: {
      title: '给我们留言',
      desc: '客服暂不在线，留下联系方式，上线后尽快联系您',
      namePlaceholder: '您的称呼（选填）',
      contactPlaceholder: '手机号或邮箱',
      contactInvalid: '请填写正确的手机号或邮箱',
      contentPlaceholder: '请描述您的问题，最多 500 字',
      submit: '提交留言',
      required: '请填写留言内容',
      submitted: '已收到您的留言，客服上线后会尽快联系您',
      failed: '留言提交失败：{reason}'
    },
    file: {
      unsupported: '不支持该文件类型，允许：{ext}',
      tooLarge: '文件大小不能超过 {size} MB',
      uploadFailed: '上传失败：{reason}',
      download: '下载',
      attachment: '附件'
    },
    message: {
      self: '我',
      agent: '客服',
      ai: 'AI 助手',
      system: '系统'
    },
    tabs: {
      chat: '对话',
      help: '帮助'
    },
    help: {
      empty: '暂无帮助内容'
    },
    typing: {
      agent: '客服正在输入…'
    },
    error: {
      sendFailed: '发送失败：{reason}',
      retry: '请重试',
      badResponse: '服务响应异常，请稍后重试',
      requestFailed: '请求失败',
      uploadNetwork: '上传失败，请检查网络后重试',
      imUrlMissing: '未配置 IM 地址',
      imNotConnected: 'IM 未连接'
    }
  }
};
