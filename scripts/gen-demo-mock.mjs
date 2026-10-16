/**
 * 游客演示模式的「mock 内容」生成 / 刷新脚本
 *
 * 为什么需要它 ✗：演示页的固定问答与 4 位角色的「分析」都是**预置内容**（不调模型），
 * 所以**大版本更新后会过期** ✗ —— 旧版本的结论（天赋顺序 / 毕业线 / 套装）会误导游客。
 * 这个脚本就是刷新流程：**拿真 AI 重新问一遍 → 人工审 → 写回库** ✓
 *
 * 两种模式：
 *   qa        —— 刷新 preset_qas 里 scene_key=guest_demo 的固定问答
 *                （问题从库里读 ⇒ 运营新加的问题也会一起刷新 ✓）
 *   analysis  —— 刷新 demo_characters 的 4 位角色「分析」
 *
 * 用法（PowerShell，先跑 scripts\tunnel-remote.ps1）：
 *   cd C:\Users\HONOR\Desktop\AI提瓦特向导
 *   $env:NODE_PATH = 'C:\Users\HONOR\Desktop\demo_in_windows\demo-app\node_modules'   # 借后台的 mysql2
 *   $env:AI_KEY       = '<主服务 config.yaml 的 ai.api_key>'        # 调 AI 服务的 X-AI-Key
 *   $env:INTERNAL_KEY = '<主服务 config.yaml 的 auth.internal_key>'  # 写 preset_qas 用（--write 时）
 *   node scripts/gen-demo-mock.mjs qa                 # 只打印草稿（默认）
 *   node scripts/gen-demo-mock.mjs qa --write         # 确认合理后写回
 *   node scripts/gen-demo-mock.mjs analysis --only=mavuika
 *
 * ⚠️ 复刻线上注入是这份脚本的**关键**（2026-10-17 踩过 ✓）：
 *   演示「分析」的 context 必须尽量与 ai_context.go / character_guide.go / character_mechanic.go 一致，
 *   尤其是【游戏资料】里的**天赋升级优先级** —— 漏了它模型会用通用记忆补，
 *   第一轮就给出与攻略卡相反的天赋建议（"玛薇卡先点普攻""茜特菈莉先点爆发"）✗✗。
 *   这里简化的地方只有一处：机制卡按 **JSON 原文** 注入（线上是渲染成散文），内容不丢 ✓。
 */
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = join(HERE, '..');

const MODE = process.argv[2];
const WRITE = process.argv.includes('--write');
const ONLY = process.argv.find((a) => a.startsWith('--only='))?.slice(7);
if (!['qa', 'analysis'].includes(MODE)) {
  console.error('用法：node scripts/gen-demo-mock.mjs qa|analysis [--write] [--only=charKey]');
  process.exit(1);
}

// ---------- 配置：优先环境变量，其次主服务 config.yaml ----------
function loadYAML(path) {
  const out = {};
  let section = '';
  for (const raw of readFileSync(path, 'utf8').split(/\r?\n/)) {
    const line = raw.replace(/#.*$/, '').replace(/\s+$/, '');
    if (!line.trim()) continue;
    const sec = line.match(/^([a-z_]+):\s*$/);
    if (sec) {
      section = sec[1];
      out[section] = out[section] ?? {};
      continue;
    }
    const kv = line.match(/^\s+([a-z_]+):\s*(.*)$/);
    if (kv && section) out[section][kv[1]] = kv[2].replace(/^["']|["']$/g, '').trim();
  }
  return out;
}
const cfg = loadYAML(join(REPO, 'server', 'config', 'config.yaml'));
const AI_BASE = process.env.AI_BASE || 'http://127.0.0.1:18090'; // 隧道口（config 里的 8090 是 ECS 内网口）
const AI_KEY = process.env.AI_KEY || '';
const INTERNAL_KEY = process.env.INTERNAL_KEY || '';
const API_BASE = process.env.API_BASE || 'http://127.0.0.1:18080';
if (!AI_KEY) {
  console.error('缺少 AI_KEY（主服务 config.yaml 的 ai.api_key）');
  process.exit(1);
}
if (!cfg.database?.host) {
  console.error('读取 server/config/config.yaml 的 database 段失败');
  process.exit(1);
}

// ---------- mysql2：本仓没装，按候选路径解析 ----------
function loadMysql() {
  const req = createRequire(join(REPO, 'package.json'));
  const candidates = [
    process.env.MYSQL2_PATH,
    'mysql2/promise',
    'C:/Users/HONOR/Desktop/demo_in_windows/demo-app/node_modules/mysql2/promise.js',
  ].filter(Boolean);
  for (const c of candidates) {
    try {
      return req(c);
    } catch {
      /* 试下一个候选 */
    }
  }
  throw new Error('没找到 mysql2：设 NODE_PATH 指向后台的 node_modules，或用 MYSQL2_PATH 指定');
}
const mysql = loadMysql();

const db = await mysql.createConnection({
  host: process.env.TEYVAT_DB_HOST || cfg.database.host,
  port: Number(process.env.TEYVAT_DB_PORT || cfg.database.port),
  user: process.env.TEYVAT_DB_USER || cfg.database.user,
  password: process.env.TEYVAT_DB_PASSWORD || cfg.database.password,
  database: process.env.TEYVAT_DB_NAME || cfg.database.name,
});

async function askAI(prompt, context) {
  const resp = await fetch(`${AI_BASE}/v1/complete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-AI-Key': AI_KEY },
    body: JSON.stringify({
      system: '',
      history: [],
      prompt,
      options: cfg.ai?.model ? { model: cfg.ai.model } : {},
      ...(context ? { context } : {}),
    }),
    signal: AbortSignal.timeout(120_000),
  });
  const payload = await resp.json();
  if (payload.code !== 0) throw new Error(`AI 服务返回失败：${payload.message}`);
  return payload.data ?? {};
}

const cut = (s, n) => String(s ?? '').replace(/\s+/g, ' ').slice(0, n);
const SLOT_CN = { sand: '时之沙', goblet: '空之杯', circlet: '理之冠' };

// ---------- 模式一：刷新固定问答 ----------
async function refreshQA() {
  const [rows] = await db.query(
    "select id, scene_key, question, answer, sort, enabled from preset_qas where scene_key = 'guest_demo' order by sort asc, id asc",
  );
  if (rows.length === 0) {
    console.log('preset_qas 里还没有 guest_demo 组；先在后台「预设问答」建几条问题，或手动插入 ✓');
    return;
  }
  for (const row of rows) {
    const d = await askAI(row.question);
    console.log(`\n########## preset_qas id=${row.id}（${row.question}）`);
    console.log(`tokens=${d.tokens?.total ?? '-'} profile=${d.profile ?? '-'} offTopic=${d.offTopic ?? '未判定'}`);
    console.log(d.reply);
    if (!WRITE) continue;
    const resp = await fetch(`${API_BASE}/internal/v1/admin/preset-qa/${row.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'X-Internal-Key': INTERNAL_KEY },
      body: JSON.stringify({
        sceneKey: row.scene_key,
        question: row.question,
        answer: d.reply,
        sort: row.sort,
        enabled: Boolean(row.enabled),
      }),
    });
    const payload = await resp.json();
    console.log(`  → 写回：${payload.code === 0 ? 'ok' : `失败 ${payload.message}`}`);
  }
}

// ---------- 模式二：刷新 4 位角色的「分析」 ----------
async function refreshAnalysis() {
  const [rows] = await db.query(
    `select char_key, name, card_json from demo_characters where enabled = 1 ${ONLY ? 'and char_key = ?' : ''} order by sort asc, id asc`,
    ONLY ? [ONLY] : [],
  );
  const [guides] = await db.query(
    'select character_id, role, talents, weapon_pick, relic_pick from character_guides',
  );
  const [mechs] = await db.query('select character_id, facts from character_mechanics');
  const guideById = new Map(guides.map((g) => [Number(g.character_id), g]));
  const mechById = new Map(mechs.map((m) => [Number(m.character_id), m]));

  for (const row of rows) {
    // mysql2 对 JSON 列会自动反序列化；两种形态都兼容 ✓
    const card = typeof row.card_json === 'string' ? JSON.parse(row.card_json) : row.card_json;
    const guide = guideById.get(Number(card.id)) ?? {};
    const mech = mechById.get(Number(card.id)) ?? {};

    const artifacts = [];
    const agg = new Map();
    for (const r of card.relics ?? []) {
      const set = r.set || r.name;
      if (!agg.has(set)) {
        agg.set(set, { set, count: 0, mainStats: {} });
        artifacts.push(agg.get(set));
      }
      const g = agg.get(set);
      g.count += 1;
      if (r.mainStat) g.mainStats[r.slot || r.pos] = r.mainStat.split(' +')[0];
    }

    const player = {
      uid: '000000000',
      nickname: '旅行者',
      level: 57,
      worldLevel: 8,
      characters: [
        {
          name: card.name,
          level: card.level,
          constellation: card.constellation,
          talents: card.talents ?? undefined,
          weapon: card.weapon
            ? { name: card.weapon.name, refine: card.weapon.affix, level: card.weapon.level }
            : undefined,
          artifacts,
          relicScore: {
            total: card.relicScoreTotal,
            avg: card.relicScoreAvg,
            class: card.relicScoreClass,
            words: card.relicWordCount,
            crit: card.relicCritScore,
            note: '单件满分66，档位按5件平均分',
            pieces: (card.relics ?? []).length,
          },
        },
      ],
      abyss: { floor: 12, room: 2, stars: 33 },
      theater: '档案中未提供（玩家可能未公开战绩）',
      stygian: '档案中未提供（玩家可能未公开战绩）',
    };

    const sw = card.statWeights ?? {};
    const sub = (sw.subStats ?? []).map((s) => s.name + (s.tier ? `(${s.tier})` : '')).join(' > ');
    const mains = Object.entries(sw.mainStats ?? {})
      .map(([k, v]) => `${SLOT_CN[k] ?? k} ${String(v).includes('/') ? v.split('/').join(' 或 ') : v}`)
      .join('，');
    let summary = `【游戏资料】（来自本服资料库，是本回合数值与词条问题的唯一依据）\n- ${card.name}：推荐副词条 ${sub}；推荐主词条 ${mains}`;
    if (guide.role) {
      // ⚠️ talents 必须带上：漏了它模型会给错的天赋建议 ✗（见文件头注释）
      summary += `\n- ${card.name} 攻略卡：定位 ${cut(guide.role, 160)}；天赋升级优先级 ${cut(guide.talents, 200)}；武器推荐 ${cut(guide.weapon_pick, 200)}；圣遗物推荐 ${cut(guide.relic_pick, 320)}`;
    }
    if (mech.facts) {
      summary += `\n- ${card.name} 机制卡：${cut(typeof mech.facts === 'string' ? mech.facts : JSON.stringify(mech.facts), 700)}`;
    }

    const d = await askAI(`帮我看看我的${card.name}练得怎么样`, {
      goal: 'character_build',
      player,
      summary,
    });
    console.log(`\n########## demo_characters ${row.char_key} / ${card.name}`);
    console.log(`tokens=${d.tokens?.total ?? '-'} profile=${d.profile ?? '-'}`);
    console.log(d.reply);
    if (!WRITE) continue;
    const [res] = await db.query('update demo_characters set analysis = ? where char_key = ?', [d.reply, row.char_key]);
    console.log(`  → 写回：影响行数=${res.affectedRows}`);
  }
}

await (MODE === 'qa' ? refreshQA() : refreshAnalysis());
await db.end();
console.log(`\n${WRITE ? '已写回库' : '仅打印草稿（加 --write 才写库）'}｜模式=${MODE}`);
