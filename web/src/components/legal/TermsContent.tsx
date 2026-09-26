/**
 * 用户须知与免责声明（正文）。
 *
 * 为什么写在组件里而不是 i18n 词典：这是**法律文本**，以中文为准，
 * 不做逐句翻译（英文界面下展示中文原文 + 一句"以中文版本为准"）。
 *
 * ⚠️ 改这里的文案 = 必须同步递增 `server/internal/service/terms.go` 的
 * `CurrentTermsVersion`，否则老用户不会重新确认（他们"同意"的是旧条款）。
 */

export const TERMS_CONTACT_EMAIL = '18241261808@163.com'
export const TERMS_AUTHOR = '黑白熊今天吃什么'

/** 与后端下发的版本号一起展示；未登录页面拿不到版本时传 undefined */
export default function TermsContent({ version }: { version?: string }) {
  return (
    <div className="space-y-3.5 text-[13px] leading-relaxed text-teyvat-text-2">
      <p className="text-xs text-teyvat-text-3">
        {version ? `版本 ${version} · ` : ''}
        生效日期 2026-09-27 · 适用站点 aiteyvat.com
      </p>

      <p className="text-teyvat-text">
        本站是一个由个人开发者搭建的
        <span className="font-semibold text-teyvat-gold">非商业性技术学习项目</span>
        ，用于学习与实践生成式 AI 应用的工程形态，供学习、评估与交流之用。本站采用
        <span className="font-semibold text-teyvat-gold">邀请制、不公开注册、不进行商业运营与推广</span>
        。请在首次使用前阅读并知悉以下条款，继续使用即视为同意。
      </p>

      <ul className="space-y-2.5">
        <Clause title="项目性质">
          本站为个人学习与技术验证项目，非官方产品，与游戏《原神》的开发商、发行商及其关联公司
          <b>没有任何关系</b>；站内不提供任何游戏账号、代充、代练、修改器或下载服务。所有分析与回答均为
          <b>技术演示与参考信息</b>，不代表任何官方立场或结论。
        </Clause>

        <Clause title="非官方声明与知识产权">
          本站为非商业性项目。站内展示的游戏名称、角色 / 装备 / 物品名称、图片与数据等，其商标、版权及其他权利
          <b>均归米哈游（miHoYo）及相关权利人所有</b>
          ，本站引用仅用于技术学习与信息展示目的，<b>无任何侵权主观故意</b>
          ，不代表任何合作、授权或背书关系。若权利人认为存在不当使用，请联系我们，我们将核实后及时下线、删除或更正。
        </Clause>

        <Clause title="数据来源与准确性">
          站内游戏数据来自<b>第三方公开接口与公开数据集</b>，可能存在延迟、缺失或与游戏内实际不一致的情况；
          AI 回答由模型生成，<b>可能存在错误或过时内容</b>。以上信息仅供参考，请以游戏内实际内容与官方公告为准。
        </Clause>

        <Clause title="UID 查询说明">
          档案查询功能仅读取游戏内<b>公开展示</b>的角色展柜数据，<b>不涉及账号、密码或任何登录凭据</b>
          ，查询结果仅用于向发起查询者展示。请勿使用本站查询他人信息或用于任何不当目的。
        </Clause>

        <Clause title="AI 生成内容标识">
          站内由 AI 生成的回答均为<b>机器生成内容</b>，已在回答处作出标识，
          <b>不得作为医疗、法律、投资、财务或其他专业决策依据</b>。
        </Clause>

        <Clause title="第三方服务与数据处理提示">
          为生成回答，您输入的内容会被发送至<b>第三方大模型 / API 服务</b>处理。一旦提交，相关内容可能离开本站，
          <b>本站无法控制其后续存储与处理</b>。请<b>不要提交任何个人隐私、账号凭据、商业秘密或受法律法规保护的信息</b>
          ，仅使用可公开的测试性内容。
        </Clause>

        <Clause title="使用规范">
          您不得利用本站生成、传播违反法律法规或公序良俗的内容，不得对本站进行攻击、爬取、逆向或批量滥用；
          因此造成的后果由使用者自行承担。
        </Clause>

        <Clause title="服务可用性不作担保">
          本站为个人项目，按<b>「现状」</b>提供，不保证服务的持续可用性、准确性、完整性或及时性，亦不保证不中断、无错误；
          本站<b>可能随时变更、暂停或终止</b>，无需事先通知。
        </Clause>

        <Clause title="责任限制">
          在适用法律允许的最大范围内，因使用或无法使用本站、或因您上传 / 提交的任何内容而产生的任何直接或间接后果、
          损失或纠纷，<b>本站及其所有者不承担责任</b>，使用本站的风险由您自行承担。
        </Clause>

        <Clause title="条款变更">
          本站保留随时修改本声明的权利；<b>重大变更后您需要重新阅读并确认</b>（版本号会随之更新）。
          继续使用即表示您已阅读、理解并接受更新后的全部条款。
        </Clause>
      </ul>

      <p className="border-t border-teyvat-line pt-3 text-xs text-teyvat-text-3">
        如对本声明有任何问题或意见，请联系：<span className="text-teyvat-text-2">{TERMS_CONTACT_EMAIL}</span>
        （个人开发者 · {TERMS_AUTHOR}）
        <br />
        For English users: the Chinese version of this notice prevails.
      </p>
    </div>
  )
}

function Clause({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <li className="relative pl-3.5 before:absolute before:top-1.5 before:left-0 before:h-1.5 before:w-1.5 before:rounded-full before:bg-teyvat-gold/70">
      <span className="font-semibold text-teyvat-text">{title}：</span>
      {children}
    </li>
  )
}
