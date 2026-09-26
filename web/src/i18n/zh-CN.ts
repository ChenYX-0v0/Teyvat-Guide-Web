// 简体中文词典（默认语言）
// key 使用「模块.名称」扁平结构；带参数的文案用 {name} 占位

const zhCN = {
  // 通用
  'common.save': '保存',
  'common.expand': '展开',
  'common.collapse': '收起',
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
  'header.menu': '菜单',
  'header.close': '关闭',
  'sidebar.home': '首页',
  'sidebar.chat': '智能问答',
  'sidebar.account': '账号信息',
  'sidebar.wiki': '图鉴',

  // 登录
  'login.welcome': '欢迎回来，旅行者',
  // 邀请制（当前部署口径，见 docs/11 上线准备 §7.6）：登录页说明，不提供注册入口
  'login.inviteOnly': '本站为不公开邀请制，仅限受邀用户登录',
  'login.inviteHint': '没有账号？请联系管理员开通。',
  // 以下 4 条为"开放注册"模式保留（当前不展示；改回 auth.allow_register=true 时可复用）
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

  // 用户须知（免责声明）——正文见 components/legal/TermsContent.tsx，
  // 版本号真源在主服务 server/internal/service/terms.go（改文案要同步递增版本）
  'terms.title': '用户须知与免责声明',
  'terms.footerLink': '用户须知',
  'terms.back': '返回',
  'terms.agree': '我已阅读并同意',
  'terms.agreeCountdown': '请阅读（{seconds}s）',
  'terms.waitSeconds': '请仔细阅读，{seconds} 秒后可确认',
  'terms.readyHint': '点击确认即表示你已阅读并同意上述全部条款',
  'terms.decline': '不同意并退出登录',
  // AI 生成内容标识（合规要求：每个 AI 回答末尾明示"机器生成、仅供参考"）
  'chat.aiGenerated': '内容由 AI 生成，仅供参考',

  // 首页
  'home.welcome': '欢迎来到提瓦特，旅行者',
  'home.feature1Title': '智能问答',
  'home.feature1Desc': '角色配队、圣遗物推荐、养成规划，一句话问清楚。',
  'home.feature2Title': '档案查询',
  'home.feature2Desc': '输入 UID 查看玩家展柜角色、养成详情，免登录免风控。',
  'home.feature3Title': '路线规划',
  'home.feature3Desc': '按目标生成采集路线，省时高效跑图。',
  'home.startChat': '开始对话',
  'home.viewWiki': '查看图鉴',
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
  'chat.send': '发送',
  // 空态示例问题（点一下即问）
  'chat.emptySample1': '新手开局怎么玩？',
  'chat.emptySample2': '万叶值得抽吗？',
  'chat.emptySample3': '给芙宁娜配队',

  // 历史会话（仅登录用户可见）
  'chat.sessions': '历史会话',
  'chat.sessionsEmpty': '还没有历史会话',
  'chat.loginHint': '登录后可保存并查看历史会话；未登录时只能看预设问答。',
  'chat.untitled': '未命名会话',
  'chat.rename': '重命名',
  'chat.renameSave': '保存',
  'chat.renameCancel': '取消',
  'chat.renameEmpty': '标题不能为空',
  'chat.renameFailed': '重命名失败，请重试',

  // 预设问答（匿名可见；登录后才能向派蒙提问）
  'chat.presetTitle': '先看看这些常见问题',
  'chat.presetHint': '未登录时只能查看预设问答；登录后派蒙可以结合你的账号与展柜角色作答。',
  'chat.presetLogin': '登录后开始对话',
  'chat.presetEmpty': '预设问答暂时取不到，稍后再试',
  'chat.loginRequired': '请先登录再向派蒙提问',
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

  // 账号页悬浮 AI 体检（角色养成度，见 web/13）
  'analysis.fab': '打开派蒙的养成度体检',
  'analysis.title': '角色养成度体检',
  'analysis.needLogin': '体检需要先登录',
  'analysis.needBinding': '先绑定游戏 UID，派蒙才能看到你的角色面板',
  'analysis.question':
    '帮我看看{name}现在的养成度怎么样：圣遗物词条对不对、武器和天赋还差什么，按优先级告诉我该先改什么。',
  'analysis.running': '正在分析 {name}…',
  'analysis.reportTitle': '{name} 的体检报告',
  'analysis.rerun': '重新分析',
  'analysis.restoring': '正在读取上次的分析…',
  'analysis.empty': '问点什么吧 —— 比如「这套圣遗物还能怎么提」',
  'analysis.retry': '重试',
  'analysis.dailyLimit': '今天的自动体检次数已用完，明天再试；下面仍可直接追问',
  'analysis.footnote': '体检会消耗你的 AI 额度 · 结论仅供参考，以最新档案为准',

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
  'character.effectiveRolls': '有效词条 {n} 条',
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
