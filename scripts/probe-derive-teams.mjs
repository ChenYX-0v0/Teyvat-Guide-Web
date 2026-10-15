#!/usr/bin/env node
/**
 * 实验：让当前模型**从角色资料推理配队**（不喂任何现成配队 ✓）
 *
 * 背景（2026-10-15 用户口径 ✓）：
 *   抓"现成配队"的路线被否 ✗（配队会过时、角色一直在更新 ✓）；
 *   用户真正想要的是 —— **根据各角色资料推导**出"谁和谁搭更合适" ✓。
 *   本脚本就是验这个问题：把「目标角色的资料」＋「候选队友池（全部角色的机制摘要）」
 *   一起给模型，看它能不能推出一支像样的队 ✓，以及**会不会瞎编** ✗。
 *
 * 与线上注入的区别 ✓（重要 ✓）：线上只注入**被问到角色**的资料 ✗，没有候选池 ⇒ 模型只能背或凭记忆 ✗；
 *   本实验额外给候选池 ✓ —— 用来判断"**加上池子**能不能推得出来" ✓（这决定要不要做这件事 ✓）。
 *
 * 用法：
 *   node scripts/probe-derive-teams.mjs 芙宁娜
 *   node scripts/probe-derive-teams.mjs 菲林斯 --pool 40        # 池子只放前 40 位（省 token ✓）
 *   node scripts/probe-derive-teams.mjs 芙宁娜 --dry            # 只打印拼好的 prompt（不调模型 ✓）
 */

import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(import.meta.dirname, '..')
const D = path.join(ROOT, 'scripts/data')
const ENV_FILE = 'C:/Users/HONOR/Desktop/AI派蒙服务器/ai-service/.env'

const arg = (k, d) => {
  const i = process.argv.indexOf('--' + k)
  if (i < 0) return d
  const v = process.argv[i + 1]
  return v && !v.startsWith('--') ? v : true
}
const TARGET = process.argv[2] && !process.argv[2].startsWith('--') ? process.argv[2] : '芙宁娜'
const POOL_LIMIT = Number(arg('pool', 0))  // 0 = 全部 ✓
const DRY = !!arg('dry', false)

// ---------- ① 读资料（就是线上用的那份 ✓）
const guides = JSON.parse(fs.readFileSync(path.join(D, 'character_guides.json'), 'utf8'))
const mech = JSON.parse(fs.readFileSync(path.join(D, 'character_mechanics.json'), 'utf8'))
const auto = JSON.parse(fs.readFileSync(path.join(D, 'character_mechanics_auto.json'), 'utf8'))

// 角色 id（用户口径 2026-10-15 ✓：**id 越大 = 越新** ✓ ⇒ 强度/出场率的弱信号 ✓）+ 用 id 时注意特例 ✗
const idsRaw = JSON.parse(fs.readFileSync(path.join(D, 'character_ids.json'), 'utf8'))
const ids = idsRaw.ids || {}

const cards = new Map()
for (const c of guides.cards || []) cards.set(c.name, c)
const mcards = new Map()
for (const c of mech.characters || []) mcards.set(c.name, c)
for (const c of auto.characters || []) if (!mcards.has(c.name)) mcards.set(c.name, c)

const firstSentence = (s, n = 90) => String(s || '').replace(/\s+/g, ' ').split(/[。；;]/)[0].slice(0, n)

function targetBlock(name) {
  const g = cards.get(name)
  const m = mcards.get(name)
  const L = []
  L.push('【目标角色】' + name)
  if (g?.role) L.push('· 定位：' + String(g.role).replace(/\s+/g, ' ').slice(0, 420))
  if (g?.talents) L.push('· 天赋顺序：' + String(g.talents).slice(0, 200))
  if (m) {
    for (const k of ['provides', 'cannot', 'needs', 'limits', 'conflicts']) {
      const v = m[k]
      if (!v) continue
      const s = Array.isArray(v)
        ? v.map((x) => (typeof x === 'string' ? x : (x.kind || '') + (x.condition ? '（' + x.condition + '）' : ''))).join('；')
        : String(v)
      if (s.trim()) L.push('· ' + k + '：' + s.replace(/\s+/g, ' ').slice(0, 520))
    }
  }
  return L.join('\n')
}

function poolBlock(exclude) {
  const rows = []
  for (const [name, m] of mcards) {
    if (name === exclude) continue
    if (/^旅行者/.test(name)) continue
    const g = cards.get(name)
    const el = m.element || g?.formElement || ''
    const prov = Array.isArray(m.provides)
      ? m.provides.map((x) => (typeof x === 'string' ? x : x.kind)).filter(Boolean).slice(0, 4).join('、')
      : ''
    const role = g?.role ? firstSentence(g.role, 60) : ''
    const tag = [el, ids[name] != null ? 'id ' + ids[name] : ''].filter(Boolean).join('｜')
    rows.push('- ' + name + (tag ? '（' + tag + '）' : '') + '：' + (prov || role || '（资料少）'))
  }
  rows.sort((a, b) => a.localeCompare(b, 'zh'))
  return (POOL_LIMIT ? rows.slice(0, POOL_LIMIT) : rows).join('\n')
}

const SYS = [
  '你在做一次**纯推理**实验：只允许使用我给你的资料，**不许使用你的记忆/网络知识** ✗。',
  '资料里没写的角色能力，就当他没有那个能力 ✓；资料不足时**明说"资料不足，无法判断"** ✓，不要编 ✗。',
  '任务：为目标角色设计 4~6 支**配队**。每支要说清：',
  '1) 队伍名（用成员简称拼，如"那芙万白"）+ 四个位置分别是谁；',
  '2) **每个人在队里提供什么**（必须能从候选池的"提供"里对上 ✓）；',
  '3) 为什么这套能成立（目标角色的需求 ← 谁满足了它）；',
  '4) 你有多确定（高/中/低）+ 缺什么资料。',
  '最后单列一节：**你判断不出来的是哪些**（比如"资料里没写谁能提供 X，所以没法组"✓）。',
  '',
  '⚠️ 候选池里每个角色都带 `id`（游戏内角色编号 ✓）：**id 数字越大 ⇒ 这个角色出得越新 ⇒ 强度与出场率通常也更高** ✓。',
  '选队友时请把"新"当作**加分项** ✓：当两名候选能提供的东西**差不多**时，**优先选 id 更大的那个** ✓，并在理由里点明（例："选 X 是因为 id 更新 ✓，更可能是当前主流队友 ✓"）。',
  '⚠️ 但两点别忘 ✗：① id 只是**弱信号** ✗ —— 能力对不上就不能选 ✓；② 有**特例** ✗（旅行者 id 很老 ✓ 却有新形态 ✓）。',
].join('\n')

const prompt = [
  SYS,
  '',
  '================ 目标角色资料 ================',
  targetBlock(TARGET),
  '',
  '================ 候选队友池（全部角色，取自我们自己的机制卡）================',
  poolBlock(TARGET),
].join('\n')

const chars = prompt.length
console.log('  目标=' + TARGET + '　prompt ' + chars + ' 字符（≈' + Math.round(chars / 1.6) + ' tokens 粗估）')
if (DRY) {
  fs.writeFileSync(process.env.TEMP + '/cb-derive-prompt.txt', prompt, 'utf8')
  console.log('  --dry：已写到 %TEMP%/cb-derive-prompt.txt\n')
  console.log(prompt.slice(0, 1200))
  process.exit(0)
}

// ---------- ② 取 DeepSeek 配置（键不外泄 ✓）
const env = {}
for (const l of fs.readFileSync(ENV_FILE, 'utf8').split(/\r?\n/)) {
  const m = l.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*?)\s*$/)
  if (m) env[m[1]] = m[2].replace(/^["']|["']$/g, '')
}
const KEY = env.DEEPSEEK_API_KEY
const BASE = (env.DEEPSEEK_BASE_URL || 'https://api.deepseek.com').replace(/\/+$/, '')
const MODEL = env.LLM_MODEL || 'deepseek-chat'
if (!KEY) { console.log('  ✗ .env 里没有 DEEPSEEK_API_KEY'); process.exit(1) }
console.log('  模型=' + MODEL + '　base=' + BASE.replace(/^https:\/\//, ''))

const t0 = Date.now()
const r = await fetch(BASE + '/chat/completions', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + KEY },
  body: JSON.stringify({
    model: MODEL,
    messages: [{ role: 'system', content: SYS }, { role: 'user', content: prompt }],
    temperature: 0.3,
  }),
})
const j = await r.json()
if (!r.ok) { console.log('  ✗ HTTP ' + r.status + '  ' + JSON.stringify(j).slice(0, 300)); process.exit(1) }
const text = j.choices?.[0]?.message?.content || ''
const usage = j.usage || {}
console.log('  用时 ' + ((Date.now() - t0) / 1000).toFixed(1) + 's　input=' + usage.prompt_tokens + ' output=' + usage.completion_tokens + ' tokens\n')
fs.writeFileSync(process.env.TEMP + '/cb-derive-answer.txt', text, 'utf8')
console.log(text)
