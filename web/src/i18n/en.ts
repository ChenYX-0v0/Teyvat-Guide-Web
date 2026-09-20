// English dictionary — structure matches zh-CN, missing keys fall back to Chinese

import type { Dict } from './types'

const en: Partial<Dict> = {
  // Common
  'common.save': 'Save',
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
  'header.subtitle': 'Teyvat Guide · Genshin AI Assistant',
  'header.logout': 'Log out',
  'header.menu': 'Menu',
  'header.close': 'Close',
  'sidebar.home': 'Home',
  'sidebar.chat': 'AI Chat',
  'sidebar.account': 'Account',
  'sidebar.wiki': 'Wiki',

  // Login
  'login.welcome': 'Welcome back, Traveler',
  // Invite-only deployment (see docs/11 §7.6): notice only, no sign-up entry
  'login.inviteOnly': 'Invite-only: sign-in is limited to invited users',
  'login.inviteHint': "Don't have an account? Ask the admin to create one for you.",
  // The 4 keys below are kept for the "open registration" mode (not shown while invite-only)
  'login.registerWelcome': 'Create your traveler account',
  'login.username': 'Username',
  'login.password': 'Password',
  'login.displayName': 'Display name (optional)',
  'login.submit': 'Sign in',
  'login.registerSubmit': 'Register & Sign in',
  'login.noAccount': "Don't have an account?",
  'login.switchToRegister': 'Register',
  'login.hasAccount': 'Already have an account?',
  'login.switchToLogin': 'Sign in',

  // Home
  'home.welcome': 'Welcome to Teyvat, Traveler',
  'home.intro':
    'Teyvat Guide is an AI assistant skeleton powered by React + Go, helping you build your own Genshin toolbox.',
  'home.feature1Title': 'Smart Q&A',
  'home.feature1Desc': 'Team building, artifact advice and leveling plans in one question.',
  'home.feature2Title': 'Profile Lookup',
  'home.feature2Desc': 'Look up any player showcase by UID — no login, no risk control.',
  'home.feature3Title': 'Route Planning',
  'home.feature3Desc': 'Generate farming routes by target, travel efficiently.',
  'home.startChat': 'Start Chatting',
  'home.viewWiki': 'Browse Wiki',
  'home.backendStatus': 'Backend: ',
  'home.statusUp': 'Up',
  'home.statusDown': 'Offline (please start the Go server)',

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
  'chat.delete': 'Delete',
  'chat.deleteConfirm': 'Delete this session?',
  'chat.deleteFailed': 'Delete failed, please try again',

  // Account
  'account.title': 'Account',
  'account.queryTitle': 'Look Up Player Profile',
  'account.queryPlaceholder': 'Enter a 9-digit game UID, e.g. 197794747',
  'account.forceRefresh': 'Force refresh (bypass local cache, query upstream)',
  'account.bind': 'Bind {uid} to my account',
  'account.bindSuccess': 'Bound! UID {uid} is now linked to your account and loads automatically.',
  'account.unbind': 'Unbind',
  'account.unbindSuccess': 'Unbound.',
  'account.boundLabel': 'Bound UID',
  'account.emptyShowcase': 'This player has no characters on display, or their info is private',

  // Player profile
  'profile.signature': 'Signature',
  'profile.statsActiveDays': 'Active Days',
  'profile.statsAchievements': 'Achievements',
  'profile.statsAvatars': 'Showcase',
  'profile.statsWorldLevel': 'World Level',
  'profile.charactersCount': 'Showcase Characters ({count})',
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
  'character.recommendTitle': 'Recommended Substats',
  'character.mainStatsLabel': 'Recommended main stats',

  // Wiki
  'wiki.title': 'Wiki',
  'wiki.placeholder': 'Placeholder module. Real data coming later.',

  // 404
  'notFound.title': '404',
  'notFound.desc': "Looks like you took a wrong turn in Stormterror's Lair",
  'notFound.back': 'Back to Home',
}

export default en
