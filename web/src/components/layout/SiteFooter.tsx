/**
 * 页脚：备案号展示（**合规必需，不要删**）。
 *
 * 依据 `teyvat-docs/web/11-上线准备（域名·备案·HTTPS）.md`：
 *   §6.4 ICP 备案号必须带网站序号 `-1`，并链接工信部 `https://beian.miit.gov.cn/`；
 *        公安联网备案号**取得后再填**（不要留占位符上线），链接公安部 `https://beian.mps.gov.cn/`；
 *   §6.5 公安审核人员会实际访问网站 → 「首页底部备案号可见、可点击、链接正确」；
 *   §6.6 备案号长期保留，网站信息变更时同步更新。
 *
 * 刻意不做成常驻底部横条之上的独立区域，而是 AppLayout 里的一条细边（不依赖滚动即可见），
 * 这样无论停在哪一页、无论屏幕多高，审核要求的那条都在。
 *
 * 文案不参与 i18n：备案号是法定中文标识，任何语言下都原样展示。
 */
import { Link } from 'react-router-dom'
import { t } from '@/i18n'

const ICP_NO = '辽ICP备2026022414号-1' // ⚠️ 必须带 -1（网站序号），漏掉不合规
const ICP_URL = 'https://beian.miit.gov.cn/'

export default function SiteFooter({ className = '' }: { className?: string }) {
  return (
    <footer
      className={
        'shrink-0 border-t border-teyvat-line px-4 py-1.5 text-center text-[11px] leading-tight text-teyvat-text-4 ' +
        className
      }
    >
      <a
        href={ICP_URL}
        target="_blank"
        rel="noopener noreferrer"
        className="transition-colors hover:text-teyvat-text-2"
      >
        {ICP_NO}
      </a>
      <span className="mx-1.5">·</span>
      <Link to="/terms" className="transition-colors hover:text-teyvat-text-2">
        {t('terms.footerLink')}
      </Link>
      {/*
        公安联网备案通过后在这里补一条（保留上面的分隔点样式），例如：
        <span className="mx-1.5">·</span>
        <a href="https://beian.mps.gov.cn/" target="_blank" rel="noopener noreferrer"
           className="transition-colors hover:text-teyvat-text-2">辽公网安备XXXXXXXX号</a>
        ⚠️ 未取得前**不要留占位符**（§6.4）。
      */}
    </footer>
  )
}
