// English dictionary — structure matches zh-CN, missing keys fall back to Chinese

import type { Dict } from './types'

const en: Partial<Dict> = {
  // Common
  'common.save': 'Save',
  'common.cancel': 'Cancel',
  'common.close': 'Close',
  'common.expand': 'Expand',
  'common.collapse': 'Collapse',
  'common.loading': 'Loading…',
  'common.query': 'Search',
  'common.querying': 'Querying…',

  // Errors
  'errors.network': 'Network error, please try again later',
  'errors.requestFailed': 'Request failed',
  'errors.unknown': 'Unknown error',
  'errors.paimonDistracted': 'Paimon got distracted, please ask again~',

  // Layout
  'header.title': 'Teyvat Guide',
  // 英文界面下标题本身就是 Teyvat Guide，副标题留空（Header 对空值不渲染，见 Header.tsx）
  'header.subtitle': '',
  'header.logout': 'Log out',
  'header.menu': 'Menu',
  'header.close': 'Close',
  'sidebar.home': 'Home',
  'sidebar.chat': 'AI Chat',
  // Bottom user card in the sidebar / drawer (moved out of the header on 2026-10-11)
  'user.menu': 'Account menu',
  'user.fallbackName': 'Traveler',
  'user.profile': 'Profile',
  'user.password': 'Change password',
  'user.feedback': 'Send feedback',
  // Profile dialog
  'profile.title': 'Profile',
  'profile.account': 'Account',
  'profile.nickname': 'Display name',
  'profile.uidLabel': 'Bound game UID',
  'profile.notBound': 'Not bound',
  // Trimmed 2026-10-11 at the user's request: usernameLabel → account ("Account"); dropped
  // nicknameHint and the goAccount / terms buttons (both already live in the nav and footer).
  'profile.saved': 'Saved',
  'profile.loadFailed': 'Could not load account info, please try again',
  // Change-password dialog
  'password.title': 'Change password',
  'password.current': 'Current password',
  'password.new': 'New password',
  'password.confirm': 'Confirm new password',
  'password.submit': 'Save new password',
  'password.ok': 'Password updated. This device stays signed in.',
  'password.mismatch': 'The two new passwords do not match',
  // 'password.otherDevices' removed 2026-10-11 at the user's request: only the on-screen notice is
  // gone, the behaviour is unchanged (changing the password still does NOT revoke tokens on other
  // devices). The boundary stays documented in ../shared/04 §2.6 and ../web/04 §3.
  // Feedback (2026-10-12): third entry in the sidebar user card; rows land in the `feedbacks` table.
  // The 5–500 range below must match server/internal/service/feedback.go (the server is authoritative).
  'feedback.title': 'Send feedback',
  'feedback.intro':
    'Found a problem, or have an idea for a feature? Write it here — it is saved under your account so we can reply.',
  'feedback.category': 'Type',
  'feedback.cat.bug': 'Something is broken',
  'feedback.cat.suggest': 'Feature request',
  'feedback.cat.experience': 'Experience / UX',
  'feedback.cat.other': 'Other',
  'feedback.content': 'Details',
  'feedback.placeholder': 'Describe the problem you hit, or the feature you would like…',
  'feedback.rule': '5–500 characters; please do not include passwords or other secrets.',
  'feedback.contact': 'Contact (optional)',
  'feedback.contactPlaceholder': 'Email or anything else, for a reply',
  'feedback.submit': 'Submit',
  'feedback.ok': 'Received — thanks for the feedback!',
  'feedback.okHint':
    'It is stored with your account; if a reply is needed we will use the contact you left.',
  'feedback.again': 'Send another',
  'feedback.tooShort': 'Too short — please write at least 5 characters',
  'sidebar.account': 'Account',
  // 'sidebar.wiki': 'Wiki',  <- commented out 2026-09-28: the wiki page is still a placeholder
  //   (no real data), so it is withdrawn from the menu. Route kept: /wiki still works if typed.

  // Login
  'login.welcome': 'Welcome back, Traveler',
  // Invite-only deployment (see docs/11 §7.6): notice only, no sign-up entry
  'login.inviteOnly': 'Invite-only: sign-in is limited to invited users',
  'login.inviteHint': "Don't have an account? Ask the admin to create one for you.",
  'login.inviteCodeNotice':
    'Registration requires a one-time invite code. Please contact the administrator for details.',
  // The 4 keys below are kept for the "open registration" mode (not shown while invite-only)
  // 'login.registerWelcome': 'Create your traveler account',  (removed 2026-10-11: no longer shown on the register form)
  'login.username': 'Username',
  'login.password': 'Password',
  'login.displayName': 'Display name (optional)',
  'login.submit': 'Sign in',
  'login.registerSubmit': 'Register & Sign in',
  'login.noAccount': "Don't have an account?",
  'login.switchToRegister': 'Register',
  'login.hasAccount': 'Already have an account?',
  'login.switchToLogin': 'Sign in',
  // Phone sign-up (2026-09-27): whether the entry shows is decided by GET /api/auth/config
  'login.inviteCode': 'Invite code',
  'login.phone': 'Phone number',
  'login.smsCode': 'SMS code',
  'login.sendCode': 'Send code',
  'login.sendingCode': 'Sending…',
  'login.resendIn': 'Resend in {seconds}s',
  'login.codeSent': 'Code sent — please check your SMS',
  'login.codeSentMock': 'Code generated (mock mode: see server logs)',
  'login.passwordRule': 'At least 6 characters',
  // 'login.registerPhoneHint': 'Sign up with your phone number, then sign in with it',  (removed 2026-10-11: no longer shown on the register form)

  // Terms & disclaimer — the declaration itself is Chinese-only by design
  // (legal text: the Chinese version prevails); version source of truth is the server.
  'terms.title': 'Terms & Disclaimer',
  'terms.footerLink': 'Terms',
  'terms.back': 'Back',
  'terms.agree': 'I have read and agree',
  'terms.agreeCountdown': 'Please read ({seconds}s)',
  'terms.waitSeconds': 'Please read carefully — you can confirm in {seconds}s',
  'terms.readyHint': 'Confirming means you have read and agree to all terms above',
  'terms.decline': 'Decline and sign out',
  // AI-generated content label (compliance: every AI answer must be marked as such)
  'chat.aiGenerated': 'AI-generated content, for reference only',

  // Home
  'home.welcome': 'Welcome to Teyvat, Traveler',
  // home.feature1~3Title/Desc (Smart Q&A / Profile Lookup / Route Planning) removed 2026-09-28:
  // they only duplicated the sidebar entries; the space is reserved for three planned cards
  // (see web/06 roadmap, P2).
  'home.startChat': 'Start Chatting',
  // 'home.viewWiki': 'Browse Wiki',  <- commented out 2026-09-28: wiki withdrawn from the menu,
  //   so the homepage no longer links to it.
  'home.backendStatus': 'Backend: ',
  'home.statusUp': 'Up',
  'home.statusDown': 'Offline (please start the Go server)',
  // Development progress: three status cards — live / in progress / planned (2026-10-16).
  // Hardcoded here (frontend, not in the database) on purpose; see zh-CN.ts for the rationale.
  'home.plan.title': 'Development progress',
  'home.plan.subtitle':
    'The site is still growing — here is what works today, what is in progress, and what is queued up.',
  'home.plan.online': 'Live',
  'home.plan.onlineItem1': 'Character build recommendations',
  'home.plan.onlineItem2': 'Sign-up & login',
  'home.plan.onlineItem3': 'Character showcase import',
  'home.plan.onlineItem4': 'Artifact scoring',
  'home.plan.onlineItem5': 'Off-topic question detection',
  'home.plan.doing': 'In progress',
  'home.plan.doingItem1': 'Team composition recommendations',
  'home.plan.doingItem2': 'In-game mechanics reference',
  'home.plan.doingItem3': 'Mobile UI improvements',
  'home.plan.planned': 'Planned',
  'home.plan.plannedItem1': 'More detailed guides for beginners',
  'home.plan.plannedItem2': 'Better chat controls',
  'home.plan.plannedItem3': 'AI timeout handling & segmented replies',
  'home.plan.plannedItem4': 'Screenshot Q&A / screenshot info import',
  'home.plan.plannedItem5': 'Team & build advice based on your own characters',
  'home.plan.plannedItem6': 'Suggested next steps',
  'home.plan.plannedItem7': 'Story / NPC recap',
  // 2026-09-28: the home page "What is coming next" roadmap block (22 home.roadmap* keys)
  // was removed — the user judged it unnecessary; see web/06 for the plan itself.
  // Chat
  'chat.title': 'AI Chat',
  'chat.subtitle': 'LLM capability proxied by the Go backend',
  'chat.new': 'New Chat',
  'chat.emptyHint': 'No messages yet. Try asking "How should a new player start?"',
  'chat.inputPlaceholder': 'Ask Paimon: e.g. "Is Kazuha worth pulling?"',
  'chat.loadEarlier': 'Load earlier messages',
  'chat.send': 'Send',
  // Empty-state sample questions (click to ask)
  'chat.emptySample1': 'How should a new player start?',
  'chat.emptySample2': 'Is Kazuha worth pulling?',
  'chat.emptySample3': 'Build a team for Furina',

  // Sessions (signed-in users only)
  'chat.sessions': 'History',
  'chat.sessionsEmpty': 'No saved sessions yet',
  'chat.loginHint':
    'Sign in to save and browse your chat history. Without signing in you can only read the preset Q&A.',
  'chat.untitled': 'Untitled session',
  'chat.rename': 'Rename',
  'chat.renameSave': 'Save',
  'chat.renameCancel': 'Cancel',
  'chat.renameEmpty': 'Title cannot be empty',
  'chat.renameFailed': 'Rename failed, please try again',

  // Preset Q&A (visible without signing in; asking questions requires sign-in)
  'chat.presetTitle': 'Try these common questions',
  'chat.presetHint':
    'Without signing in you can only read preset Q&A. Sign in and Paimon can answer based on your account and showcase characters.',
  'chat.presetLogin': 'Sign in to start chatting',
  'chat.presetEmpty': 'Preset Q&A is unavailable right now, please try again later',
  'chat.loginRequired': 'Please sign in before asking Paimon',
  // 2026-10-12: chat-page banner "Paimon cannot see your characters" (reason + next step)
  'chat.ctxNotBound': 'No game UID linked yet, so Paimon cannot see your characters.',
  'chat.ctxNoProfile': 'Your showcase is empty or private, so Paimon cannot see your characters.',
  'chat.ctxAction': 'Open account page',
  'chat.delete': 'Delete',
  'chat.deleteConfirm': 'Delete this session?',
  'chat.deleteFailed': 'Delete failed, please try again',

  // Account
  'account.title': 'Account',
  'account.queryTitle': 'Look Up Player Profile',
  'account.queryPlaceholder': 'Enter a 9-digit game UID, e.g. 197794747',
  // 'account.forceRefresh' removed 2026-09-28 (profiles always use the server snapshot; no bypass switch in the UI)
  // 'account.bind' removed 2026-10-12 (option A): a successful lookup now auto-binds server-side,
  // so the UI no longer needs a second "bind" click. The backend endpoint POST /api/genshin/binding
  // is KEPT for old cached bundles that still call it.
  'account.queryAutoBindHint':
    'A successful lookup binds this UID to your account (unbind or switch it inside "Bound UID").',
  'account.bindSuccess': 'Bound! UID {uid} is now linked to your account and loads automatically.',
  'account.unbind': 'Unbind',
  'account.unbindSuccess': 'Unbound.',
  'account.boundLabel': 'Bound UID',
  'account.emptyShowcase': 'This player has no characters on display, or their info is private',

  // Account-page floating AI build check (see web/13)
  'analysis.fab': 'Open Paimon build check',
  'analysis.title': 'Build check',
  'analysis.needLogin': 'Please sign in to run a build check',
  'analysis.needBinding': 'Bind your game UID first so Paimon can see your characters',
  'analysis.question':
    'How is my {name} build doing? Check the artifact stats, weapon and talents, and tell me what to improve first, in priority order.',
  'analysis.running': 'Analyzing {name}…',
  'analysis.reportTitle': 'Build report for {name}',
  'analysis.rerun': 'Re-analyze',
  'analysis.restoring': 'Loading the last analysis…',
  'analysis.empty': 'Ask anything — e.g. "How can I improve these artifacts?"',
  'analysis.retry': 'Retry',
  'analysis.dailyLimit': 'Daily auto-check limit reached; try again tomorrow (you can still ask below)',
  'analysis.footnote':
    'Checks use your AI quota · Suggestions are for reference only, based on the latest profile',

  // Player profile
  'profile.signature': 'Signature',
  'profile.statsActiveDays': 'Active Days',
  'profile.statsAchievements': 'Achievements',
  'profile.statsAvatars': 'Showcase',
  'profile.statsWorldLevel': 'World Level',
  // 'profile.charactersCount' ("Showcase Characters (N)") reworked 2026-10-11: the strip now renders the
  // accumulated pool, so it uses the two keys below (total collected + current showcase).
  'profile.collectedCount': 'Collected Characters ({count})',
  'profile.showcaseHint': '{showcase} in showcase now · swap it to import more — nothing is dropped',
  'profile.outOfShowcaseHint': 'Not in showcase · data as of {time} (put it back to refresh)',
  'profile.noSignature': 'This traveler left nothing here',
  'profile.partialFailed': 'Some name mappings are missing and queued for update',
  'profile.statsTitle': 'Character Stats',
  'character.weaponLabel': 'Weapon',

  // Character card
  'character.constellation': 'C{n}',
  'character.refine': 'R{n}',
  'character.fetter': 'Fetter Lv.{n}',
  'character.relicsCount': '{n} Artifacts',
  'character.effectiveRolls': '{n} effective rolls',
  'character.enhanceTimes': 'Enhanced {n}x',
  'character.relicScore': 'Relic score',
  'character.relicScoreNote': '66 max per piece, graded D/C/B/A/S/SS/SSS/ACE/MAX by 5-piece average',
  'character.relicWords': '{n} rolls',
  'character.relicCrit': 'Crit {n}',
  'character.recommendTitle': 'Recommended Substats',
  'character.mainStatsLabel': 'Recommended main stats',

  // Guest demo mode (2026-10-17, public /demo and /demo/chat)
  // The demo content itself (character cards / "analysis") is NOT in the dictionary: it lives in the
  // database so it can be edited without a release. Only the UI copy lives here.
  'demo.badge': 'Demo',
  'demo.loginCta': 'Sign in / Sign up',
  'demo.loginEntry': 'See the demo first (no sign-up)',
  'demo.tabAccount': 'Demo profile',
  'demo.tabChat': 'Demo chat',
  'demo.tabHome': 'Demo home',
  'demo.homeCta': 'Try the chat',
  'demo.homeHint':
    'Demo mode: the profile and chat below are preset samples — no AI calls, no quota used.',
  'demo.guestName': 'Traveler (guest)',
  'demo.noticeFallback':
    'This is demo content: the character profile and the analysis are preset samples, not live account data.',
  'demo.accountTitle': 'Demo profile',
  'demo.accountCtaHint':
    'After you sign in and bind your UID, Paimon sees your own character profile.',
  'demo.analysisNote': 'Preset sample · no quota used',
  'demo.chatTitle': 'Demo chat',
  'demo.chatHint':
    'These Q&As are preset: tap one to watch Paimon think and type the answer out. The demo makes no AI calls and uses no quota.',
  'demo.chatPick': 'Pick a question',
  'demo.chatLocked': 'Sign in to ask anything (your own profile / teams / builds)',
  'demo.chatReset': 'Start over',
  'demo.empty': 'Demo content is not ready yet, please try again later.',
  'demo.asideHint':
    'Demo mode: everything here is a preset sample. Sign in to see your own characters.',
  'demo.asideTerms': 'Terms of use',
  'demo.sessionHint':
    'Demo mode keeps no chat history. After you sign in, your conversations show up here.',
  'demo.analysisCta': 'Sign in to analyse your own characters',

  // Wiki
  'wiki.title': 'Wiki',
  'wiki.placeholder': 'Placeholder module. Real data coming later.',

  // 404
  'notFound.title': '404',
  'notFound.desc': "Looks like you took a wrong turn in Stormterror's Lair",
  'notFound.back': 'Back to Home',
}

export default en
