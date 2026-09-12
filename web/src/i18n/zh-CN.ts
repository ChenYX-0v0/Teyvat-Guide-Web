// 简体中文词典（默认语言）
// key 使用「模块.名称」扁平结构；带参数的文案用 {name} 占位

const zhCN = {
  // 通用
  'common.save': '保存',
  'common.loading': '加载中…',
  'common.query': '查询',
  'common.querying': '查询中…',

  // 错误信息
  'errors.network': '网络异常，请稍后重试',
  'errors.requestFailed': '请求失败',
  'errors.unknown': '未知错误',
  'errors.paimonDistracted': '派蒙走神了，请再问一次～',

  // 布局
  'header.title': 'AI 提瓦特向导',
  'header.subtitle': 'Teyvat Guide · 原神智能助手机',
  'header.logout': '退出登录',
  'sidebar.home': '首页',
  'sidebar.chat': '智能问答',
  'sidebar.account': '账号信息',
  'sidebar.wiki': '图鉴',

  // 登录
  'login.welcome': '欢迎回来，旅行者',
  'login.registerWelcome': '创建你的旅行者账号',
  'login.username': '用户名',
  'login.password': '密码',
  'login.displayName': '昵称（可选）',
  'login.submit': '登录',
  'login.registerSubmit': '注册并登录',
  'login.noAccount': '还没有账号？',
  'login.switchToRegister': '注册',
  'login.hasAccount': '已有账号？',
  'login.switchToLogin': '登录',

  // 首页
  'home.welcome': '欢迎来到提瓦特，旅行者',
  'home.intro':
    'AI 提瓦特向导是一个由 React + Go 驱动的智能助手骨架，帮你快速搭建属于自己的原神工具站。',
  'home.feature1Title': '智能问答',
  'home.feature1Desc': '角色配队、圣遗物推荐、养成规划，一句话问清楚。',
  'home.feature2Title': '档案查询',
  'home.feature2Desc': '输入 UID 查看玩家展柜角色、养成详情，免登录免风控。',
  'home.feature3Title': '路线规划',
  'home.feature3Desc': '按目标生成采集路线，省时高效跑图。',
  'home.startChat': '开始对话',
  'home.backendStatus': '后端状态：',
  'home.statusUp': '正常',
  'home.statusDown': '未连接（请先启动 Go 服务）',

  // 智能问答
  'chat.title': '智能问答',
  'chat.subtitle': '由 Go 后端统一代理大模型能力',
  'chat.new': '新对话',
  'chat.emptyHint': '还没有对话，试着问一句「新手开局怎么玩？」',
  'chat.inputPlaceholder': '问问派蒙吧：比如「万叶值得抽吗？」',
  'chat.loadEarlier': '加载更早的消息',

  // 历史会话（仅登录用户可见）
  'chat.sessions': '历史会话',
  'chat.sessionsEmpty': '还没有历史会话',
  'chat.loginHint': '登录后可保存并查看历史会话；当前是匿名对话，刷新页面即重新开始。',
  'chat.untitled': '未命名会话',
  'chat.rename': '重命名',
  'chat.renameSave': '保存',
  'chat.renameCancel': '取消',
  'chat.renameEmpty': '标题不能为空',
  'chat.renameFailed': '重命名失败，请重试',
  'chat.delete': '删除',
  'chat.deleteConfirm': '删除这条会话？',
  'chat.deleteFailed': '删除失败，请重试',

  // 账号信息
  'account.title': '账号信息',
  'account.queryTitle': '查询玩家档案',
  'account.queryPlaceholder': '输入 9 位游戏 UID，如 197794747',
  'account.forceRefresh': '强制刷新（绕过本地缓存，直接查询上游）',
  'account.bind': '绑定 {uid} 到我的账号',
  'account.bindSuccess': '绑定成功，UID {uid} 已与账号一对一关联，下次进入自动加载。',
  'account.unbind': '解绑',
  'account.unbindSuccess': '已解除绑定。',
  'account.boundLabel': '已绑定 UID',
  'account.emptyShowcase': '该玩家未摆放角色展柜，或未公开详细信息',

  // 玩家档案
  'profile.signature': '签名',
  'profile.statsActiveDays': '活跃天数',
  'profile.statsAchievements': '成就达成',
  'profile.statsAvatars': '展柜角色',
  'profile.statsWorldLevel': '世界等级',
  'profile.charactersCount': '展柜角色（{count}）',
  'profile.noSignature': '这个人很懒，什么都没写',
  'profile.partialFailed': '部分名称映射缺失，已记录待补录',
  'profile.statsTitle': '角色面板',
  'character.weaponLabel': '武器',

  // 角色卡片
  'character.constellation': '{n}命',
  'character.refine': '精{n}',
  'character.fetter': '好感 Lv.{n}',
  'character.relicsCount': '圣遗物 {n} 件',
  'character.enhanceTimes': '强化 {n} 次',
  'character.recommendTitle': '推荐强化词条',
  'character.mainStatsLabel': '主词条推荐',

  // 图鉴
  'wiki.title': '图鉴',
  'wiki.placeholder': '该页面为占位模块，后续接入真实数据。',

  // 404
  'notFound.title': '404',
  'notFound.desc': '这条风龙废墟的入口似乎走错了',
  'notFound.back': '回到首页',
}

export default zhCN
