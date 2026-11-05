/** 接口统一返回结果 */
export interface ApiResult<T = unknown> {
  /** 状态码, 0 表示成功 */
  code: number;
  /** 状态信息 */
  message?: string;
  /** 返回数据 */
  data?: T;
}

/** 分页查询统一结果 */
export interface PageResult<T> {
  /** 返回数据 */
  list: T[];
  /** 总数量 */
  count: number;
}

/** 分页查询基本参数 */
export interface PageParam {
  /** 第几页 */
  page?: number;
  /** 每页多少条 */
  limit?: number;
}

/** 渠道公开配置 */
export interface ChannelConfig {
  /** 渠道标识 */
  channelCode: string;
  /** 渠道类型 */
  channelType: string;
  /** 渠道名称 */
  name: string;
  /** 欢迎语 */
  welcomeText: string;
  /** 主题色 */
  themeColor: string;
  /** 是否必须登录后咨询, 0否, 1是 */
  needLogin: number;
  /** 聊天窗地址, 悬浮脚本据此拼面板地址 */
  chatUrl?: string;
  /** 访客附件上传开关 */
  fileEnabled?: boolean;
  /** 单文件大小上限, 单位 MB */
  fileMaxSizeMb?: number;
  /** 允许上传的附件后缀, 不含点号 */
  fileAllowedExt?: string[];
  /** 访客端「转人工」按钮开关 */
  handoffEnabled?: boolean;
  /** 会话评价开关 */
  rateEnabled?: boolean;
}

/** 访客快捷问题，聊天窗只展示标题与内容 */
export interface QuickReplyItem {
  /** 快捷回复id */
  quickReplyId: number;
  /** 标题 */
  title: string;
  /** 内容 */
  content: string;
}

/** 附件上传结果 */
export interface UploadedFile {
  /** 文件记录id */
  fileId: number;
  /** 文件名称 */
  name: string;
  /** 文件大小, 单位字节 */
  size: number;
  /** 附件访问地址 */
  url: string;
  /** 文件类型 */
  contentType: string;
}

/** 访客接入表单 */
export interface BootstrapForm {
  /** 渠道标识 */
  channelCode: string;
  /** 访客标识, 首次接入时为空 */
  visitorKey?: string;
  /** 昵称, 选填 */
  nickname?: string;
  /** 邮箱, 选填 */
  email?: string;
  /** 来源页面地址 */
  sourceUrl?: string;
}

/** 访客资料, 预填与修改时使用, 只包含昵称与邮箱 */
export interface VisitorProfile {
  /** 昵称 */
  nickname?: string;
  /** 邮箱 */
  email?: string;
}

/** 访客接入结果 */
export interface BootstrapResult {
  /** 访客的 IM 用户标识 */
  uid: string;
  /** 访客的 IM token */
  token: string;
  /** IM gateway 地址 */
  wsUrl: string;
  /** AI 服务号的 UID */
  aiUid: string;
  /** 访客id */
  visitorId: number;
  /** 访客标识, 首次接入时由服务端生成 */
  visitorKey: string;
  /** 访客昵称 */
  nickname: string;
  /** 访客邮箱, 未填时为空 */
  email: string;
  /** 当前会话id */
  sessionId: number;
  /** 会话状态, 0排队中, 1进行中, 2已结束, 3留言 */
  sessionStatus: number;
  /** 接待客服的名称, 未分配时为空 */
  agentName: string;
  /** AI 自动回复开关, 0关闭, 1开启 */
  aiMode: number;
  /** 欢迎语 */
  welcomeText: string;
}

/** 访客 */
export interface Visitor {
  /** 访客id */
  visitorId: number;
  /** 昵称 */
  nickname: string;
  /** 头像 */
  avatar: string;
  /** 手机号 */
  phone: string;
  /** 邮箱, 选填 */
  email: string;
}

/** 会话 */
export interface Session {
  /** 会话id */
  sessionId: number;
  /** 会话编号 */
  sessionNo: string;
  /** 渠道标识 */
  channelCode: string;
  /** 客服id, 0 表示还未分配 */
  agentId: number;
  /** 会话状态, 0待接待, 1 AI 进行中, 2已结束, 3留言, 4人工进行中 */
  status: number;
  /** AI 自动回复开关, 0关闭, 1开启 */
  aiMode: number;
  /** 排队名次, 从 1 开始, 只在待接待时有值 */
  queuePosition?: number;
  /** 待接待会话总数 */
  queueSize?: number;
  /** 预计等待秒数, 按排队名次估算 */
  estimatedWaitSeconds?: number;
  /** 已等待秒数, 从进入待接待算起 */
  queueWaitSeconds?: number;
  /** 满意度评分, 1-5, 0 表示未评价 */
  rateScore?: number;
  /** 满意度标签, 英文逗号分隔 */
  rateTags?: string;
  /** 满意度备注 */
  rateRemark?: string;
  /** 评价时间 */
  rateTime?: string;
  /** 访客信息 */
  visitor?: Visitor;
  /** 接待客服信息, 已分配时由服务端回填 */
  agent?: {
    /** 客服昵称 */
    nickname?: string;
    /** 客服账号 */
    username?: string;
  };
}

/** 访客主动转人工的结果 */
export interface HandoffResult {
  /** 会话id */
  sessionId: number;
  /** 会话状态, 转人工后为待接待 */
  status: number;
  /** 是否有人工客服在线, 无人在线时前端直接展示留言卡片 */
  onlineAgent: boolean;
}

/** 会话评价表单 */
export interface RateForm {
  /** 满意度评分, 1-5 */
  score: number;
  /** 满意度标签 */
  tags: string[];
  /** 备注, 最长 200 字 */
  remark: string;
}

/** 离线留言表单 */
export interface LeaveMessageForm {
  /** 访客姓名 */
  name: string;
  /** 联系方式类型, phone 手机号, email 邮箱 */
  contactType: string;
  /** 联系方式 */
  contact: string;
  /** 留言内容, 最长 500 字 */
  content: string;
}

/** 会话消息 */
export interface Message {
  /** 消息id */
  messageId: number;
  /** 会话id */
  sessionId: number;
  /** IM 侧幂等消息编号 */
  clientMsgNo: string;
  /** 发送者角色, visitor/agent/ai/system */
  fromRole: string;
  /** 内容类型, text/image/file/system/ai_answer */
  contentType: string;
  /** 文本内容 */
  content: string;
  /** 是否为 AI 生成 */
  aiGenerated: number;
  /** 关联的 AI 回复id */
  aiReplyId: number;
  /** 消息原始 payload, json 文本 */
  payload?: string;
  /** 创建时间 */
  createdAt?: string;
  /** 创建时间 */
  createTime: string;
}

/** IM 消息的业务协议, 对应服务端的 im.Payload */
export interface ChatPayload {
  /** 协议版本 */
  version: number;
  /** 内容类型, text/image/file/system/ai_answer */
  type: string;
  /** 发送者角色, visitor/agent/ai/system */
  role: string;
  /** 文本内容 */
  content: string;
  /** 业务消息id */
  messageId?: number;
  /** 会话id */
  sessionId: number;
  /** 附加信息 */
  meta?: Record<string, unknown>;
}
