import { useState, type ReactNode } from "react"
import {
  ArrowUp, CaretRight, File, FileArchive, FileAudio, FileCode, FileImage, FilePdf,
  FileVideo, Folder, FolderOpen, MagnifyingGlass,
} from "@phosphor-icons/react"
import type { Entry, Location } from "./api"

type Props = {
  title: string
  subtitle: string
  icon: ReactNode
  entries: Entry[]
  selected: string[]
  location: Location[]
  goTo: (index: number) => void
  goUp: () => void
  canGoUp: boolean
  open: (entry: Entry) => void
  select: (path: string, additive: boolean) => void
  selectAll: (paths: string[]) => void
  actions?: ReactNode
  children?: ReactNode
  busy?: boolean
  error?: string
  drop?: boolean
  showHidden: boolean
}

const collation = new Intl.Collator("zh-CN", { numeric: true, sensitivity: "base" })
const date = new Intl.DateTimeFormat("zh-CN", { month: "short", day: "numeric" })

export function byteSize(bytes: number) {
  const unit = ["B", "KB", "MB", "GB", "TB"]
  const index = bytes > 0 ? Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 4) : 0
  return (bytes / 1024 ** index).toLocaleString("zh-CN", { maximumFractionDigits: index === 0 ? 0 : 1 }) + " " + unit[index]
}

function FileIcon({ entry }: { entry: Entry }) {
  if (entry.directory) return <Folder size={19} weight="fill" className="file-folder" />
  const ext = entry.name.toLowerCase().split(".").pop()
  switch (ext) {
    case "png": case "jpeg": case "jpg": case "heic": case "webp": case "gif":
      return <FileImage size={19} className="file-image" />
    case "zip": case "gz": case "rar": case "7z":
      return <FileArchive size={19} className="file-archive" />
    case "mp4": case "mov": case "mkv":
      return <FileVideo size={19} className="file-image" />
    case "mp3": case "m4a": case "wav":
      return <FileAudio size={19} className="file-image" />
    case "pdf": return <FilePdf size={19} className="file-pdf" />
    case "js": case "py": case "json": case "go":
      return <FileCode size={19} className="file-archive" />
    default: return <File size={19} className="file-generic" />
  }
}

export function Files(props: Props) {
  const [query, setQuery] = useState("")
  const [sort, setSort] = useState<"name" | "modified" | "size">("name")
  const [ascending, setAscending] = useState(true)
  const visible = props.entries.filter(e => (props.showHidden || !e.name.startsWith(".")) &&
    e.name.toLocaleLowerCase().includes(query.toLocaleLowerCase().trim())).sort((a, b) => {
      if (a.directory !== b.directory) return a.directory ? -1 : 1
      const result = sort === "name" ? collation.compare(a.name, b.name) : a[sort] - b[sort]
      return (ascending ? result : -result) || collation.compare(a.name, b.name)
    })
  const all = visible.length > 0 && visible.every(e => props.selected.includes(e.path))
  const changeSort = (field: typeof sort) => {
    setAscending(field === sort ? !ascending : field === "name")
    setSort(field)
  }
  return (
    <section className="file-pane" aria-label={props.title} data-file-drop-target={props.drop ? "android" : undefined}>
      <header className="pane-head">
        <span className="pane-icon">{props.icon}</span>
        <div className="pane-title"><h2>{props.title}</h2><p>{props.subtitle}</p></div>
        <div className="pane-actions">{props.actions}</div>
      </header>
      {props.children ? props.children : <>
        <div className="path-row">
          <button className="icon-btn path-up" onClick={props.goUp} title="上一级" aria-label="上一级" disabled={!props.canGoUp}><ArrowUp size={16} /></button>
          <div className="crumbs" aria-label="当前路径">
            {props.location.map((location, i) => <span key={location.path}>
              {i > 0 && <CaretRight size={12} />}
              <button onClick={() => props.goTo(i)} aria-current={i === props.location.length - 1 ? "location" : undefined}>{location.name}</button>
            </span>)}
          </div>
          <label className="file-search"><MagnifyingGlass size={16}/><input type="search" value={query} onChange={e => setQuery(e.target.value)} placeholder="搜索" aria-label={props.title + "内搜索"}/></label>
        </div>
        <div className="file-scroll" aria-busy={props.busy}>
          <table className="file-table">
            <thead><tr>
              <th className="select-col"><input type="checkbox" aria-label="全选当前列表" checked={all} disabled={visible.length === 0} onChange={() => props.selectAll(all ? [] : visible.map(e => e.path))}/></th>
              {([["name","名称"], ["modified","修改日期"], ["size","大小"]] as const).map(([field,label]) => <th key={field} className={"col-" + field} aria-sort={sort === field ? (ascending ? "ascending" : "descending") : "none"}><button onClick={() => changeSort(field)}>{label}{sort === field && <ArrowUp size={11} className={!ascending ? "sort-desc" : ""}/>}</button></th>)}
            </tr></thead>
            <tbody>
              {visible.map(entry => <tr key={entry.path} data-selected={props.selected.includes(entry.path)} onClick={e => props.select(entry.path, e.metaKey || e.ctrlKey || e.shiftKey)} onDoubleClick={() => props.open(entry)}>
                <td><input type="checkbox" aria-label={"选择 " + entry.name} checked={props.selected.includes(entry.path)} onClick={e => e.stopPropagation()} onChange={() => props.select(entry.path,true)}/></td>
                <td className="name-cell"><button className="entry-name" title={entry.name} onClick={e => { if (e.detail === 0) props.open(entry) }}><FileIcon entry={entry}/><span>{entry.name}</span>{entry.link && <span className="link-marker">链接</span>}</button></td>
                <td className="date-cell">{entry.modified ? date.format(entry.modified * 1000) : "—"}</td>
                <td className="size-cell">{entry.directory ? "—" : byteSize(entry.size)}</td>
              </tr>)}
            </tbody>
          </table>
          {visible.length === 0 && <div className="list-empty">
            <FolderOpen size={34} weight="light" />
            <strong>{props.busy ? "正在读取…" : props.error ? "无法读取文件夹" : query ? "没有匹配的项目" : "文件夹为空"}</strong>
            {props.error && <p>{props.error}</p>}
          </div>}
        </div>
        <div className="pane-foot"><span>{visible.length} 个项目</span>{props.selected.length > 0 && <span>{props.selected.length} 个已选</span>}</div>
      </>}
    </section>
  )
}
