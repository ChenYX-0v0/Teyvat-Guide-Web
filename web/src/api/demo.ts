import { http, unwrap } from '@/api/client'
import type { DemoShowcase } from '@/types/demo'

/**
 * 游客演示模式（2026-10-17）—— `/demo` 两页的数据来源。
 *
 * 契约（主服务 `server/internal/handler/demo_showcase.go`，统一信封 `{code,message,data}`）：
 *   GET /api/demo/showcase → data { profile, characters[], notice }
 *
 * 三条口径（与后端 `service/demo_showcase.go` 一致 ✓）：
 * 1. **无需登录** ✓ —— 演示页就是给"还没注册的人"看的；
 * 2. **不调模型 / 不消耗 token / 不查 Enka / 不落库会话** ✓ —— 服务端只发库里预置的内容，
 *    演示页的"思考 + 逐字输出"由前端本地回放 ✓（见 `pages/DemoChatPage`）；
 * 3. 演示数据与真实档案分表 ✓ —— 这条链路永远读不到、也污染不了真实用户数据 ✓。
 */
export const demoApi = {
  showcase() {
    return unwrap<DemoShowcase>(http.get('/demo/showcase'))
  },
}
