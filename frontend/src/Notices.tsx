import { useCallback, useEffect, useRef, useState } from "react"
import { CheckCircle, Info, WarningCircle, X } from "@phosphor-icons/react"

type Kind = "success" | "error" | "info"
type Notice = { id: number; kind: Kind; title: string; description?: string }

const icons = { success: CheckCircle, error: WarningCircle, info: Info }

export function useNotices() {
  const nextId = useRef(0)
  const [notices, setNotices] = useState<Notice[]>([])

  const notify = useCallback((kind: Kind, title: string, description?: string) => {
    const notice: Notice = { id: ++nextId.current, kind, title, description }
    setNotices(current => [...current.slice(-3), notice])
  }, [])

  const dismiss = useCallback((id: number) => {
    setNotices(current => current.filter(notice => notice.id !== id))
  }, [])

  return { notices, notify, dismiss }
}

function NoticeCard({ notice, dismiss }: { notice: Notice; dismiss: (id: number) => void }) {
  const [hovered, setHovered] = useState(false)
  const [focused, setFocused] = useState(false)
  const Icon = icons[notice.kind]

  useEffect(() => {
    if (notice.kind === "error" || hovered || focused) return
    const timer = window.setTimeout(() => dismiss(notice.id), 5000)
    return () => window.clearTimeout(timer)
  }, [notice, dismiss, hovered, focused])

  return <div className="notice" data-kind={notice.kind}
    onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}
    onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}>
    <Icon className="notice-icon" size={21} weight="duotone" aria-hidden="true"/>
    <div className="notice-content">
      <strong>{notice.title}</strong>
      {notice.description && <p>{notice.description}</p>}
    </div>
    <button type="button" className="icon-btn" aria-label={"关闭通知：" + notice.title}
      onClick={() => dismiss(notice.id)}><X size={16} aria-hidden="true"/></button>
  </div>
}

export function NoticeStack({ notices, dismiss }: { notices: Notice[]; dismiss: (id: number) => void }) {
  const latest = notices.at(-1)
  const message = latest && [latest.title, latest.description].filter(Boolean).join("。")

  return <>
    <div className="visually-hidden" role="status" aria-atomic="true">
      {latest && latest.kind !== "error" && <span key={latest.id}>{message}</span>}
    </div>
    <div className="visually-hidden" role="alert" aria-atomic="true">
      {latest?.kind === "error" && <span key={latest.id}>{message}</span>}
    </div>
    {notices.length > 0 && <aside className="notice-stack" aria-label="操作通知">
      {notices.map(notice => <NoticeCard key={notice.id} notice={notice} dismiss={dismiss}/>)}
    </aside>}
  </>
}
