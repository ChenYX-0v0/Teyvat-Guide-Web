// 简体中文词典（默认语言）
// key 使用「模块.名称」扁平结构；带参数的文案用 {name} 占位

const zhCN = {
  // 通用
  'common.save': '保存',
  'common.cancel': '取消',
  'common.close': '关闭',
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
  'header.subtitle': 'Teyvat Guide',
  'header.logout': '退出登录',
  'header.menu': '菜单',
  'header.close': '关闭',
  'sidebar.home': '首页',
  'sidebar.chat': '智能问答',
  // 侧栏/抽屉底部的用户卡片（2026-10-11：原先在顶部栏的昵称+退出，移到这里）
  'user.menu': '账号菜单',
  'user.fallbackName': '旅行者',
  'user.profile': '个人信息管理',
  'user.password': '修改密码',
  'user.feedback': '功能反馈',
  // 个人信息管理弹窗
  'profile.title': '个人信息管理',
  'profile.account': '账号',
  'profile.nickname': '昵称',
  'profile.uidLabel': '已绑定游戏 UID',
  'profile.notBound': '未绑定',
  // 2026-10-11 按用户要求精简：'profile.usernameLabel'（用户名（登录用））→ 'profile.account'（账号）；
  // 并去掉 'profile.nicknameHint'（昵称下方那行说明小字）与 'profile.goAccount' / 'profile.terms'
  // （底部两个按钮）—— 角色绑定与条款在左侧导航、页脚另有入口，不必在这个小弹窗里再兜一圈 ✓
  'profile.saved': '已保存',
  'profile.loadFailed': '读取账号信息失败，请稍后重试',
  // 修改密码弹窗
  'password.title': '修改密码',
  'password.current': '当前密码',
  'password.new': '新密码',
  'password.confirm': '确认新密码',
  'password.submit': '保存新密码',
  'password.ok': '密码已修改，当前设备无需重新登录。',
  'password.mismatch': '两次输入的新密码不一致',
  // 'password.otherDevices'（"其他设备不会被自动退出…"）已于 2026-10-11 按用户要求移除 ✓
  // ⚠️ 去掉的只是**这行界面提示** ✗ —— 后端行为一点没变（改密仍不吊销其他设备已签发的 Token ✓）；
  // 这条边界仍记在 ../shared/04 §2.6 与 ../web/04 §3，别因为界面不提醒就以为已经踢下线了 ✗。
  // 功能反馈（2026-10-12）：侧栏用户卡片的第三个功能，提交后落库 feedbacks 专表 ✓
  // ⚠️ 'feedback.rule' 里的 5~500 必须与 server/internal/service/feedback.go 的
  // feedbackMinRunes / feedbackMaxRunes **同一组数字** ✓ —— 前端只是提示，真正把关在服务端 ✓
  'feedback.title': '功能反馈',
  'feedback.intro': '遇到问题、或有想加的功能，都可以写在这里 —— 会记在你的账号名下，方便我们回复。',
  'feedback.category': '类型',
  'feedback.cat.bug': '功能异常',
  'feedback.cat.suggest': '功能建议',
  'feedback.cat.experience': '体验意见',
  'feedback.cat.other': '其他',
  'feedback.content': '具体内容',
  'feedback.placeholder': '请描述你遇到的问题、或者想要的功能…',
  'feedback.rule': '5~500 字；请勿填写密码等敏感信息。',
  'feedback.contact': '联系方式（选填）',
  'feedback.contactPlaceholder': '方便回复时留（邮箱 / 其他）',
  'feedback.submit': '提交',
  'feedback.ok': '已收到，感谢你的反馈！',
  'feedback.okHint': '内容会记在你的账号名下；需要回复时我们会用你留的联系方式联系你。',
  'feedback.again': '再提一条',
  'feedback.tooShort': '内容太短了，请至少写 5 个字',
  'sidebar.account': '账号信息',
  // 'sidebar.wiki': '图鉴',  ← 2026-09-28 注释掉：图鉴页仍是占位（无真实数据），先从菜单撤下 ✗
  //   （路由保留 ⇒ 直接输 /wiki 仍可访问 ✓；做真实数据时把 navItems.ts 那一行与这两条文案一起恢复 ✓）

  // 登录
  'login.welcome': '欢迎回来，旅行者',
  // 邀请制（当前部署口径，见 docs/11 上线准备 §7.6）：登录页说明，不提供注册入口
  'login.inviteOnly': '本站为不公开邀请制，仅限受邀用户登录',
  'login.inviteHint': '没有账号？请联系管理员开通。',
  'login.inviteCodeNotice': '该网站需使用一次性邀请码进行注册，相关功能请咨询管理员。',
  // 以下 4 条为"开放注册"模式保留（当前不展示；改回 auth.allow_register=true 时可复用）
  // 'login.registerWelcome': '创建你的旅行者账号',  （2026-10-11 去掉：注册页不再重复这句标题）
  'login.username': '用户名',
  'login.password': '密码',
  'login.displayName': '昵称（可选）',
  'login.submit': '登录',
  'login.registerSubmit': '注册并登录',
  'login.noAccount': '还没有账号？',
  'login.switchToRegister': '注册',
  'login.hasAccount': '已有账号？',
  'login.switchToLogin': '登录',
  // 手机号注册（2026-09-27）：注册入口是否显示由服务端 `/api/auth/config` 决定 ✓
  'login.inviteCode': '邀请码',
  'login.phone': '手机号',
  'login.smsCode': '短信验证码',
  'login.sendCode': '获取验证码',
  'login.sendingCode': '发送中…',
  'login.resendIn': '{seconds} 秒后重发',
  'login.codeSent': '验证码已发送，请查收短信',
  'login.codeSentMock': '验证码已生成（联调模式：验证码见服务端日志）',
  'login.passwordRule': '密码至少 6 位',
  // 'login.registerPhoneHint': '用手机号注册，注册后可直接用手机号登录',  （2026-10-11 去掉：注册页不再重复这句标题）

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
  // 原 home.feature1~3Title/Desc（智能问答 / 档案查询 / 路线规划）已于 2026-09-28 移除 ✓
  // —— 那三张卡只是重复了左侧导航的入口；腾出的位置留给规划中的三张新卡（见 web/06 路线图 P2）
  'home.startChat': '开始对话',
  // 'home.viewWiki': '查看图鉴',  ← 2026-09-28 注释掉：图鉴已从菜单撤下，首页不再挂它的入口 ✗
  'home.backendStatus': '后端状态：',
  'home.statusUp': '正常',
  'home.statusDown': '未连接（请先启动 Go 服务）',
  // 开发进度三态卡（2026-10-16 恢复并升级 ✓；2026-09-28 的首版是「正在做 / 计划中」两态，同日撤掉 ✓）
  // ⚠️ 内容**硬编码在本字典**（即前端，不入库）✓ —— 记的是"开发进度"这类产品文案，
  //    不是资产映射 / 枚举那类业务数据，与 `AGENTS.md` §4.4 不冲突 ✓（2026-09-28 首版同样硬编码 ✓）。
  // ⚠️ 措辞一律**使用者视角** ✓（不写 RAG / PWA / lint 之类内部词 ✗）；后台与工程类任务不进这里 ✗。
  'home.plan.title': '开发进度',
  'home.plan.subtitle': '站点还在长身体 —— 下面是已经能用的、正在做的、以及排上队的。',
  'home.plan.online': '已上线',
  'home.plan.onlineItem1': '角色养成推荐',
  'home.plan.onlineItem2': '账号注册登录体系',
  'home.plan.onlineItem3': '角色拉取',
  'home.plan.onlineItem4': '圣遗物评分',
  'home.plan.onlineItem5': '无关问题识别',
  'home.plan.doing': '开发中',
  'home.plan.doingItem1': '角色配队推荐',
  'home.plan.doingItem2': '游戏内机制资料',
  'home.plan.doingItem3': '移动端 UI 优化',
  'home.plan.planned': '待排期',
  'home.plan.plannedItem1': '面向新手的更详细解读',
  'home.plan.plannedItem2': '对话操作增强',
  'home.plan.plannedItem3': 'AI 超时处理与分段回复',
  'home.plan.plannedItem4': '截图提问 / 截图信息导入',
  'home.plan.plannedItem5': '按现有角色推荐配队与养成',
  'home.plan.plannedItem6': '优先处理事项推荐',
  'home.plan.plannedItem7': '剧情回顾 / NPC 回顾',
  // 2026-09-28：曾加过首页「接下来会有什么」开发计划板块（22 个 home.roadmap* 文案键），
  // 用户看过觉得没必要 ⇒ 整块撤掉 ✓（组件里的 ROADMAP 数组也一并删了）。要恢复就翻这次提交的反向 diff ✓
  // 智能问答
  'chat.title': '智能问答',
  'chat.subtitle': '由 Go 后端统一代理大模型能力',
  'chat.new': '新对话',
  'chat.emptyHint': '还没有对话，试着问一句「新手开局怎么玩？」',
  'chat.inputPlaceholder': '问问派蒙吧：比如「万叶值得抽吗？」',
  'chat.loadEarlier': '加载更早的消息',
  'chat.send': '发送',
  'chat.stop': '停止生成',
  'chat.stopped': '已停止生成',
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
  // 2026-10-12：聊天页「派蒙看不到你的角色」提示条（说清原因 + 给出下一步 ✓）
  'chat.ctxNotBound': '还没绑定游戏 UID，派蒙看不到你的角色。',
  'chat.ctxNoProfile': '你的展柜是空的或未公开，派蒙暂时看不到角色数据。',
  'chat.ctxAction': '去账号页处理',
  'chat.delete': '删除',
  'chat.deleteConfirm': '删除这条会话？',
  'chat.deleteFailed': '删除失败，请重试',

  // 账号信息
  'account.title': '账号信息',
  'account.queryTitle': '查询玩家档案',
  'account.queryPlaceholder': '输入 9 位游戏 UID，如 197794747',
  // 'account.forceRefresh' 已于 2026-09-28 移除（档案一律走服务端快照，前端不再暴露绕过缓存的开关 ✓）
  // 'account.bind'（"绑定 {uid} 到我的账号"）已于 2026-10-12 移除 ✗ ——
  // 现在是**查询成功即自动绑定**（口径 A ✓），界面上不再需要"第二次点击"的绑定入口 ✓。
  // ⚠️ 后端 `POST /api/genshin/binding` **仍然保留** ✓：老 bundle（浏览器缓存）还在发它 ✗。
  'account.queryAutoBindHint': '查询成功后会把这个 UID 绑定到你的账号（可在展开「已绑定 UID」后解绑或换绑）。',
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
  // 'profile.charactersCount'（「展柜角色（N）」）已于 2026-10-11 改造：角色条改为渲染**累积池**，
  // 所以改用下面两条（已收录总数 + 本轮展柜数）。两个口径都要露出来，用户才知道"换展柜还能再导入" ✓
  'profile.collectedCount': '已收录角色（{count}）',
  'profile.showcaseHint': '本轮展柜 {showcase} 位 · 换展柜可继续导入，已收录的不会丢',
  'profile.outOfShowcaseHint': '已在展柜外 · 数据截至 {time}（换回展柜即可刷新）',
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
  'character.relicScore': '圣遗物评分',
  'character.relicScoreNote': '单件满分 66，按 5 件平均分档 D/C/B/A/S/SS/SSS/ACE/MAX',
  'character.relicWords': '词条 {n}',
  'character.relicCrit': '双暴 {n}',
  'character.recommendTitle': '推荐强化词条',
  'character.mainStatsLabel': '主词条推荐',

  // 游客演示模式（2026-10-17，免登录公开页 /demo 与 /demo/chat）
  // ⚠️ 演示内容（角色卡 / 「分析」）**不在词典里** ✗ —— 它是运营在库里预置的数据（改库即可改文案 ✓），
  // 这里只放界面自身的文案 ✓。
  'demo.badge': '演示',
  'demo.loginCta': '登录 / 注册',
  'demo.loginEntry': '先看看演示（免注册）',
  'demo.tabAccount': '演示档案',
  'demo.tabChat': '演示对话',
  'demo.tabHome': '演示首页',
  'demo.homeCta': '直接试试对话',
  'demo.homeHint': '演示模式：下面的档案与对话都是预置示例，不联网、不消耗额度。',
  'demo.guestName': '旅行者（游客）',
  'demo.noticeFallback': '以下为演示内容：角色档案与「分析」都是预置示例，不是实时账号数据。',
  'demo.accountTitle': '演示档案',
  'demo.accountCtaHint': '登录并绑定 UID 后，派蒙看到的就是你自己角色的真实档案。',
  'demo.analysisNote': '预置示例 · 不消耗额度',
  'demo.chatTitle': '演示对话',
  'demo.chatHint': '下面几条都是预置问答：点一条看派蒙「思考 + 逐字回答」的过程。演示不联网、不消耗额度。',
  'demo.chatPick': '选一条问题试试',
  'demo.chatLocked': '登录后可自由提问（你的角色档案 / 配队 / 养成）',
  'demo.chatReset': '重来',
  'demo.empty': '演示内容还没准备好，请稍后再试。',
  'demo.asideHint': '演示模式：这里的内容都是预置示例；登录后可看你自己角色的真实档案。',
  'demo.asideTerms': '用户须知',
  'demo.sessionHint': '演示模式不保存会话。登录后这里会显示你的历史对话。',
  'demo.analysisCta': '登录后给自己的角色做体检',

  // 图鉴
  'wiki.title': '图鉴',
  'wiki.placeholder': '该页面为占位模块，后续接入真实数据。',

  // 404
  'notFound.title': '404',
  'notFound.desc': '这条风龙废墟的入口似乎走错了',
  'notFound.back': '回到首页',
}

export default zhCN
