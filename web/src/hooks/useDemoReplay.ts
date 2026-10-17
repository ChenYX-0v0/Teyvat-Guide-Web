import { useCallback, useEffect, useRef, useState } from 'react'

export type DemoReplayPhase = 'idle' | 'thinking' | 'typing'

interface Options {
  /** "思考"（气泡呼吸点）时长 */
  thinkMs?: number
  /** 逐字输出的节拍 */
  tickMs?: number
  /** 每拍吐几个字（中文 2 字 / 24ms ≈ 每秒 80 字，接近真人打字速度 ✓） */
  charsPerTick?: number
}

interface Handlers {
  /** 每吐出一段就回调，拿到"当前已输出的完整文本" ✓ */
  onSlice: (text: string) => void
  onDone?: () => void
}

/**
 * 演示模式的**本地回放**（2026-10-17 抽出，见 TODO-32 ✓）：先在气泡里"想"一会儿，再逐字吐答案。
 *
 * 为什么要有它 ✓：`/demo` 是给还没注册的人看的 ✓，但匿名调模型＝任何人白嫖额度（口径见
 * `shared/05-AI使用边界设计` §1 ✗）。所以演示页的答案本来就存在库里，前端**只用计时器**把它演出来 ——
 * **零请求、不消耗 token、不查 Enka、不落库会话** ✓。
 *
 * 抽成 hook 的唯一理由：对话页（若干条气泡）与档案页（分析面板里那一条）要**同一套节奏** ✓，
 * 各写一遍必然漂移 ✗。
 */
export function useDemoReplay({ thinkMs = 900, tickMs = 24, charsPerTick = 2 }: Options = {}) {
  const [phase, setPhase] = useState<DemoReplayPhase>('idle')
  const timers = useRef<{ think?: number; type?: number }>({})
  // 回调放 ref：计时器闭包捕获的是**播放那一刻**的函数，直接调用会读到旧的 state ✗
  const handlers = useRef<Handlers | null>(null)

  /** 停掉当前回放（幂等 ✓；连点问题时先调它，避免两次输出叠在同一条气泡上 ✗） */
  const stop = useCallback(() => {
    if (timers.current.think) window.clearTimeout(timers.current.think)
    if (timers.current.type) window.clearInterval(timers.current.type)
    timers.current = {}
    handlers.current = null
    setPhase('idle')
  }, [])

  const play = useCallback(
    (full: string, next: Handlers) => {
      stop()
      handlers.current = next
      setPhase('thinking')
      timers.current.think = window.setTimeout(() => {
        timers.current.think = undefined
        setPhase('typing')
        let printed = 0
        timers.current.type = window.setInterval(() => {
          printed = Math.min(full.length, printed + charsPerTick)
          handlers.current?.onSlice(full.slice(0, printed))
          if (printed >= full.length) {
            if (timers.current.type) window.clearInterval(timers.current.type)
            timers.current = {}
            setPhase('idle')
            handlers.current?.onDone?.()
            handlers.current = null
          }
        }, tickMs)
      }, thinkMs)
    },
    [stop, thinkMs, tickMs, charsPerTick],
  )

  // 卸载即停：计时器还活着会在已卸载的组件上 setState
  useEffect(() => stop, [stop])

  return { phase, play, stop, busy: phase !== 'idle' }
}
