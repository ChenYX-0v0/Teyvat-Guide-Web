import { http, unwrap } from '@/api/client'

/**
 * 功能反馈（2026-10-12）—— 侧栏用户卡片「功能反馈」提交的用户意见。
 *
 * 契约（主服务 `server/internal/handler/feedback.go`，统一信封 `{code,message,data}`）：
 *   POST /api/feedback   body { category?, content, contact? } → data { id, createdAt }
 *
 * 三条口径：
 * 1. **需登录** ✓ —— 反馈一定要能回溯到人（`user_id` 由服务端从 Token 取 ✓，请求体里**没有** userId ✗）；
 * 2. 长度与类别由服务端定档 ✓（**按字符** 5~500、类别走白名单 ✓）：前端那份提示只为 UX ✓，
 *    真正的把关在服务端 ✓（前端两套规则必然漂移 ✗）；
 * 3. 服务端**不回显内容** ✓（提交成功只回 id 与时间），所以界面不假装能"查看已提交内容" ✗。
 */
export type FeedbackCategory = 'bug' | 'suggest' | 'experience' | 'other'

export type FeedbackInput = {
  category: FeedbackCategory
  /** 反馈正文（服务端按字符限长 5~500 ✓） */
  content: string
  /** 联系方式（选填 ✓，方便回复） */
  contact?: string
}

/** 提交结果：只回 id 与时间（服务端刻意不回显内容 ✓） */
export type FeedbackCreated = {
  id: number
  createdAt: string
}

export const feedbackApi = {
  submit(input: FeedbackInput) {
    return unwrap<FeedbackCreated>(http.post('/feedback', input))
  },
}
