import { useCallback, useEffect, useRef, useState, type ReactNode } from "react"
import { createRoot } from "react-dom/client"
import {
  AndroidLogo, ArrowsClockwise, ArrowsLeftRight, ArrowDown, ArrowRight, ArrowUp, CheckCircle, CircleNotch, Desktop,
  DownloadSimple, Folder, FolderOpen, FolderPlus, HardDrives, Info, LinkSimple, ListBullets,
  Plugs, Plus, Question, PencilSimple, SlidersHorizontal, Trash, UploadSimple, Usb, WifiHigh as Wifi, X,
} from "@phosphor-icons/react"
import { Events } from "@wailsio/runtime"
import { api, deviceKey, errorText, modeLabel, type Device, type Direction, type Entry, type Location, type Machine, type Progress, type Scan, type WiFiService } from "./api"
import { Files } from "./Files"
import relaydroidIcon from "./assets/relaydroid-icon.png"
import { NoticeStack, useNotices } from "./Notices"
import "./styles.css"

type Page = "files" | "devices" | "transfers"
type DialogKind = "wifi" | "help" | "settings" | "folder" | "rename" | "delete"
type Task = Progress & { direction: Direction; state: "running" | "done" | "error" | "cancelled"; error?: string }
type Listing = { entries: Entry[]; busy: boolean; error?: string }

function useListing(loader: (() => Promise<Entry[]>) | null, revision: number) {
  const [listing, setListing] = useState<Listing & { loader?: typeof loader }>({ entries: [], busy: false })
  useEffect(() => {
    let current = true
    setListing({ entries: [], busy: loader !== null, loader })
    if (loader) loader().then(entries => {
      if (current) setListing({ entries: entries ?? [], busy: false, loader })
    }).catch(error => {
      if (current) setListing({ entries: [], busy: false, error: errorText(error), loader })
    })
    return () => { current = false }
  }, [loader, revision])
  return listing.loader === loader ? listing : { entries: [], busy: loader !== null }
}

function toggle(values: string[], item: string, additive: boolean) {
  if (!additive) return [item]
  return values.includes(item) ? values.filter(value => value !== item) : [...values, item]
}

function IconButton({ title, children, onClick, disabled }: { title: string; children: ReactNode; onClick: () => void; disabled?: boolean }) {
  return <button className="icon-btn" title={title} aria-label={title} disabled={disabled} onClick={onClick}>{children}</button>
}

function Modal({ title, onClose, busy, children }: { title: string; onClose: () => void; busy: boolean; children: ReactNode }) {
  const dialog = useRef<HTMLDialogElement>(null)
  useEffect(() => { dialog.current?.showModal() }, [])
  return <dialog ref={dialog} className="modal" aria-labelledby="dialog-title" onCancel={event => { event.preventDefault(); if (!busy) onClose() }}>
    <div className="modal-heading"><h2 id="dialog-title">{title}</h2><IconButton title="关闭" onClick={onClose} disabled={busy}><X size={19}/></IconButton></div>
    {children}
  </dialog>
}

function DeviceArt() {
  return <div className="device-art" aria-hidden="true">
    <span className="art-orbit art-orbit-outer"/><span className="art-orbit art-orbit-inner"/>
    <span className="art-node art-node-mac"><Desktop size={22} weight="duotone"/></span>
    <span className="art-node art-node-usb"><Usb size={20} weight="duotone"/></span>
    <span className="art-phone"><i/><AndroidLogo size={32} weight="fill"/></span>
    <span className="art-spark art-spark-1"/><span className="art-spark art-spark-2"/>
  </div>
}

function App() {
  const { notices, notify, dismiss } = useNotices()
  const [page, setPage] = useState<Page>("files")
  const [machine, setMachine] = useState<Machine>()
  const [scan, setScan] = useState<Scan>()
  const [scanning, setScanning] = useState(false)
  const scanRunning = useRef(false)
  const scanVersion = useRef(0)
  const [preferred, setPreferred] = useState<Device>()
  const [local, setLocal] = useState<Location[]>([])
  const [remoteTrail, setRemoteTrail] = useState<{ key: string; locations: Location[] }>()
  const [localSelected, setLocalSelected] = useState<string[]>([])
  const [remoteSelected, setRemoteSelected] = useState<string[]>([])
  const [localRev, refreshLocal] = useState(0)
  const [remoteRev, refreshRemote] = useState(0)
  const [dialog, setDialog] = useState<DialogKind>()
  const [dialogBusy, setDialogBusy] = useState(false)
  const [formError, setFormError] = useState("")
  const [name, setName] = useState("")
  const [wifiStep, setWifiStep] = useState<"pair" | "connect" | "legacy">("pair")
  const [wifiServices, setWiFiServices] = useState<WiFiService[]>([])
  const [discovering, setDiscovering] = useState(false)
  const [legacyConsent, setLegacyConsent] = useState(false)
  const legacyOrigin = useRef<Device>(undefined)
  const [endpoint, setEndpoint] = useState("")
  const [code, setCode] = useState("")
  const [showHidden, setShowHidden] = useState(() => localStorage.getItem("relay.hidden") === "yes")
  const [mounted, setMounted] = useState(false)
  const [mountBusy, setMountBusy] = useState(false)
  const [tasks, setTasks] = useState<Task[]>([])
  const running = useRef<string | null>(null)
  const cancelRequested = useRef(false)

  const choices = scan?.devices ?? []
  const connected = choices.filter(d => d.state === "device")
  const active = preferred
    ? connected.find(d => deviceKey(d) === deviceKey(preferred))
    : connected.find(d => d.kind !== "mtp") ??
      (choices.some(d => d.kind === "adb-usb") ? undefined : connected[0])
  const key = active ? deviceKey(active) : ""
  const owner = active ?? preferred
  const ownerKey = owner ? deviceKey(owner) : ""
  const usb = active?.kind === "adb-usb" ? active : connected.find(d => d.kind === "adb-usb")
  const stableDevice = useRef(active)
  stableDevice.current = active
  const localPath = local.at(-1)?.path ?? ""
  const remote = remoteTrail?.key === ownerKey
    ? remoteTrail.locations
    : owner ? [{ name: owner.kind === "mtp" ? "存储空间" : "内部存储", path: owner.root }] : []
  const remotePath = remote.at(-1)?.path ?? ""
  const activeTask = tasks.find(task => task.state === "running")
  const hasRemoteFolder = !!active && (active.kind !== "mtp" || remotePath !== "/")
  const currentDeviceLabel = owner?.name ?? "Android"
  const localTitle = machine?.os === "darwin" ? "这台 Mac" : "这台电脑"
  const suggested = wifiServices.filter(service => service.pairing === (wifiStep === "pair"))
  const compatibleWiFiService = suggested.some(service => service.endpoint === endpoint) ? endpoint : ""

  const rescan = useCallback(async (manual = false) => {
    if (scanRunning.current) return
    scanRunning.current = true
    const version = scanVersion.current
    if (manual) setScanning(true)
    try {
      const snapshot = await api.scan()
      if (version === scanVersion.current) setScan(snapshot)
    } catch (error) {
      if (manual) notify("error", "无法刷新设备", errorText(error))
    } finally {
      scanRunning.current = false
      setScanning(false)
    }
  }, [notify])

  useEffect(() => {
    api.machine().then(machine => {
      setMachine(machine)
      const start = machine.places.find(place => place.name === "下载") ?? machine.places[0]
      if (start) setLocal(start.path === machine.home ? [start] : [machine.places[0], start])
    }).catch(error => notify("error", "无法读取本机目录", errorText(error)))
    void rescan(true)
    const timer = window.setInterval(() => void rescan(), 2500)
    return () => window.clearInterval(timer)
  }, [notify, rescan])

  useEffect(() => {
    if (active) setPreferred(previous => previous ?? active)
  }, [active])

  useEffect(() => {
    setRemoteSelected([])
  }, [ownerKey])

  const localLoader = useCallback(() => api.local(localPath), [localPath])
  const remoteLoader = useCallback(() => {
    const device = stableDevice.current
    return device && deviceKey(device) === key ? api.remote(device, remotePath) : Promise.resolve([])
  }, [key, remotePath, active?.transport])
  const localFiles = useListing(localPath ? localLoader : null, localRev)
  const remoteFiles = useListing(key && remotePath ? remoteLoader : null, remoteRev)
  const remoteTarget = remoteFiles.entries.find(entry => entry.path === remoteSelected[0])

  const goLocal = (trail: Location[]) => { setLocal(trail); setLocalSelected([]) }
  const goRemote = (trail: Location[]) => { setRemoteTrail({ key: ownerKey, locations: trail }); setRemoteSelected([]) }
  const goPlace = (place: Location) => {
    if (!machine) return
    goLocal(place.path === machine.home ? [place] : [machine.places[0], place])
    setPage("files")
  }
  const pickLocal = async () => {
    try {
      const [path] = await api.choose(localPath, true, false) ?? []
      if (path) goLocal([{ name: path.split(/[/\\]/).filter(Boolean).at(-1) ?? path, path }])
    } catch (error) { notify("error", "无法打开文件夹", errorText(error)) }
  }
  const upLocal = async () => {
    if (local.length > 1) return goLocal(local.slice(0, -1))
    try {
      const path = await api.parent(localPath)
      if (path !== localPath) goLocal([{ name: path.split(/[/\\]/).filter(Boolean).at(-1) ?? path, path }])
    } catch (error) { notify("error", errorText(error)) }
  }

  const discoverWiFi = useCallback(async () => {
    setDiscovering(true)
    try { setWiFiServices(await api.wifiServices() ?? []) }
    catch { setWiFiServices([]) }
    finally { setDiscovering(false) }
  }, [])

  const changeWiFiStep = (step: typeof wifiStep) => {
    setWifiStep(step)
    setEndpoint("")
    setCode("")
    setFormError("")
    setLegacyConsent(false)
    if (step !== "legacy") void discoverWiFi()
  }

  const openDialog = (kind: DialogKind) => {
    setFormError("")
    if (kind === "wifi") changeWiFiStep(scan?.legacy ? "legacy" : "pair")
    if (kind === "folder") setName("")
    if (kind === "rename") setName(remoteTarget?.name ?? "")
    setDialog(kind)
  }

  const transfer = useCallback(async (direction: Direction, sources: string[]) => {
    if (running.current) { notify("info", "当前传输结束后可继续"); return }
    const device = stableDevice.current
    if (!device || !sources.length || !localPath || !remotePath) return
    if (direction === "upload" && device.kind === "mtp" && remotePath === "/") {
      notify("info", "请先打开手机上的存储空间")
      return
    }
    const id = typeof crypto.randomUUID === "function" ? crypto.randomUUID() : String(Date.now())
    const total = sources.length
    running.current = id
    cancelRequested.current = false
    setTasks(list => [{ id, direction, current: "", done: 0, total, state: "running" }, ...list].slice(0, 50) as Task[])
    try {
      await api.transfer({ id, direction, device, sources, destination: direction === "upload" ? remotePath : localPath })
      setTasks(list => list.map(task => task.id === id ? { ...task, done: total, state: "done" } : task))
      notify("success", direction === "upload" ? "已发送到 Android" : "已保存到电脑", total + " 个项目")
    } catch (error) {
      const detail = errorText(error)
      const cancelled = cancelRequested.current || detail.includes("context canceled")
      setTasks(list => list.map(task => task.id === id ? { ...task, state: cancelled ? "cancelled" : "error", error: detail } : task))
      if (cancelled) notify("info", "传输已停止")
      else notify("error", "部分或全部项目传输失败", detail)
    } finally {
      running.current = null
      if (direction === "upload") refreshRemote(value => value + 1)
      else refreshLocal(value => value + 1)
    }
  }, [localPath, remotePath, notify])

  useEffect(() => Events.On("relay:progress", (event) => {
    const progress = event.data as Progress
    setTasks(list => list.map(task => task.id === progress.id && task.state === "running" ? { ...task, ...progress } : task))
  }), [])
  useEffect(() => Events.On("relay:drop", event => {
    if (page === "files" && Array.isArray(event.data)) void transfer("upload", event.data as string[])
  }), [page, transfer])

  const chooseUpload = async (folders: boolean) => {
    try {
      const selected = await api.choose(localPath, folders, true)
      if (selected?.length) await transfer("upload", selected)
    } catch (error) { notify("error", "无法选择上传项目", errorText(error)) }
  }

  const cancel = async (id: string) => {
    try { cancelRequested.current = true; await api.cancel(id) }
    catch (error) { notify("error", "无法停止传输", errorText(error)) }
  }

  const mountInFinder = async () => {
    if (!active && !mounted) return
    setMountBusy(true)
    try {
      if (mounted) {
        await api.finderUnmount()
        setMounted(false)
        notify("success", "已推出 Finder 中的设备")
      } else if (active) {
        await api.finderMount(active)
        setMounted(true)
        notify("success", "已在 Finder 中挂载", "可在 Finder 浏览和拷出文件。写入请使用 Relaydroid。")
      }
    } catch (error) { notify("error", "Finder 操作失败", errorText(error)) }
    finally { setMountBusy(false) }
  }

  const selectDevice = (device: Device) => {
    if (device.state !== "device" || running.current) return
    setPreferred(device)
    setScan(previous => previous ? { ...previous, devices: [device, ...previous.devices.filter(d => deviceKey(d) !== deviceKey(device))] } : previous)
    setPage("files")
  }

  const endLegacy = async () => {
    const id = scan?.legacy
    await api.stopLegacy()
    scanVersion.current++
    if (owner?.id === id) setPreferred(legacyOrigin.current ?? usb)
    setScan(previous => previous ? { ...previous, legacy: undefined, devices: previous.devices.filter(d => d.id !== id) } : previous)
    notify("success", "已关闭手机上的临时无线连接")
    void rescan()
  }

  const disconnectDevice = async (device: Device) => {
    try {
      if (device.id === scan?.legacy) return await endLegacy()
      await api.disconnect(device.id)
      scanVersion.current++
      if (deviceKey(device) === ownerKey) setPreferred(usb)
      setScan(previous => previous ? { ...previous, devices: previous.devices.filter(d => deviceKey(d) !== deviceKey(device)) } : previous)
      void rescan()
    } catch (error) { notify("error", "无法断开连接", errorText(error)) }
  }

  const submit = async () => {
    setDialogBusy(true)
    setFormError("")
    try {
      if (dialog === "wifi") {
        if (wifiStep === "legacy" && scan?.legacy) {
          await endLegacy()
          setDialog(undefined)
          return
        }
        let device: Device | undefined
        if (wifiStep === "legacy") {
          if (!usb || !legacyConsent) return
          legacyOrigin.current = usb
          device = await api.startLegacy(usb)
          setScan(previous => previous ? { ...previous, legacy: device?.id } : previous)
        } else if (wifiStep === "pair") {
          const result = await api.pair(endpoint.trim(), code.trim())
          device = result.device
          setCode("")
          if (!device) {
            notify("success", "已完成配对，请连接设备")
            setWifiStep("connect")
            setEndpoint(result.endpoint ?? "")
            void discoverWiFi()
          }
        } else {
          device = await api.connect(endpoint.trim())
        }
        if (device) {
          scanVersion.current++
          selectDevice(device)
          notify("success", "已连接 Wi-Fi")
          setDialog(undefined)
        }
        return
      }
      if (!active || activeTask) return
      if (dialog === "folder") await api.mkdir(active, remotePath, name)
      if (dialog === "rename" && remoteTarget && name !== remoteTarget.name) await api.rename(active, remoteTarget.path, name)
      if (dialog === "delete" && remoteTarget) await api.remove(active, remoteTarget.path)
      setRemoteSelected([])
      refreshRemote(value => value + 1)
      setDialog(undefined)
      notify("success", { folder: "文件夹已创建", rename: "已重命名", delete: "已删除" }[dialog as "folder" | "rename" | "delete"])
    } catch (error) { setFormError(errorText(error)) }
    finally {
      setDialogBusy(false)
      if (dialog === "wifi") void rescan()
    }
  }

  const close = () => setDialog(undefined)
  const external = (target: string) => void api.openURL(target).catch(error => notify("error", errorText(error)))
  const activateLocal = (entry: Entry) => entry.directory ? goLocal([...local, { name: entry.name, path: entry.path }]) : void api.reveal(entry.path).catch(error => notify("error", errorText(error)))
  const activateRemote = (entry: Entry) => entry.directory ? goRemote([...remote, { name: entry.name, path: entry.path }]) : setRemoteSelected([entry.path])

  const navItems = [
    { id: "files", icon: FolderOpen, label: "文件浏览" },
    { id: "devices", icon: HardDrives, label: "设备连接" },
    { id: "transfers", icon: ListBullets, label: "传输记录" },
  ] as const

  return <div className="app" data-os={machine?.os ?? (navigator.userAgent.includes("Mac") ? "darwin" : "")}>
    <aside className="sidebar" aria-label="侧栏">
      <div className="brand"><img className="brand-mark" src={relaydroidIcon} alt=""/><span>Relaydroid</span></div>
      <nav className="nav">
        {navItems.map(({ id, icon: Icon, label }) => <button key={id} className={page === id ? "nav-current" : ""} aria-current={page === id ? "page" : undefined} onClick={() => setPage(id)}>
          <Icon size={19} weight={page === id ? "duotone" : "regular"}/><span>{label}</span>{id === "transfers" && activeTask && <span className="nav-dot"/>}
        </button>)}
      </nav>
      <div className="side-places">
        <p>电脑上的位置</p>
        {machine?.places.map(place => <button key={place.path} onClick={() => goPlace(place)} data-current={page === "files" && localPath === place.path}>
          {place.name === "下载" ? <DownloadSimple size={17}/> : place.name === "桌面" ? <Desktop size={17}/> : <Folder size={17}/>}
          <span>{place.name}</span>
        </button>)}
      </div>
      <div className="side-bottom">
        <button onClick={() => openDialog("settings")}><SlidersHorizontal size={18}/><span>偏好设置</span></button>
        <button onClick={() => openDialog("help")}><Question size={18}/><span>连接帮助</span></button>
      </div>
    </aside>

    <main className="workspace">
      <div className="chrome" aria-hidden="true"><span>Relaydroid</span></div>
      <header className="workspace-header">
        <div><h1>{navItems.find(item => item.id === page)?.label}</h1><p>{page === "files" ? "在电脑和 Android 之间传输文件" : page === "devices" ? "通过 USB 或 Wi-Fi 连接 Android" : "查看本次打开应用以来的传输"}</p></div>
        <div className="workspace-actions">
          <span className="connection-status" data-online={!!active}><i/>{active ? modeLabel(active.kind) : owner ? "等待设备重连" : "设备未连接"}</span>
          <button className="button primary" onClick={() => openDialog("wifi")} disabled={!!activeTask}><Plus size={17} weight="bold"/>连接设备</button>
        </div>
      </header>

      {page === "files" && <div className="file-workspace">
        <div className="panes">
          <Files title={localTitle} subtitle="本地文件" icon={<Desktop size={22} weight="duotone"/>}
            {...localFiles} selected={localSelected} location={local} goTo={i => goLocal(local.slice(0, i + 1))} goUp={() => void upLocal()}
            canGoUp={!!localPath} open={activateLocal} select={(item, additive) => setLocalSelected(list => toggle(list, item, additive))}
            selectAll={setLocalSelected} showHidden={showHidden}
            actions={<>
              <IconButton title="选择本机文件夹" onClick={() => void pickLocal()}><FolderOpen size={19}/></IconButton>
              <IconButton title="刷新本机文件" onClick={() => { setLocalSelected([]); refreshLocal(value => value + 1) }}><ArrowsClockwise size={17}/></IconButton>
            </>}/>
          <Files title={currentDeviceLabel} subtitle={owner ? modeLabel(owner.kind) : "设备文件"} icon={<AndroidLogo size={22} weight="duotone"/>}
            {...remoteFiles} selected={remoteSelected} location={remote} goTo={i => goRemote(remote.slice(0, i + 1))}
            goUp={() => goRemote(remote.slice(0, -1))} canGoUp={remote.length > 1} open={activateRemote}
            select={(item, additive) => setRemoteSelected(list => toggle(list, item, additive))} selectAll={setRemoteSelected}
            showHidden={showHidden} drop={hasRemoteFolder}
            actions={active ? <>
              <IconButton title="上传文件" disabled={!hasRemoteFolder || !!activeTask} onClick={() => void chooseUpload(false)}><UploadSimple size={19}/></IconButton>
              <IconButton title="上传文件夹" disabled={!hasRemoteFolder || !!activeTask} onClick={() => void chooseUpload(true)}><Folder size={18}/></IconButton>
              <IconButton title="新建文件夹" disabled={!hasRemoteFolder || !!activeTask} onClick={() => openDialog("folder")}><FolderPlus size={19}/></IconButton>
              <IconButton title="重命名" disabled={!hasRemoteFolder || !remoteTarget || remoteSelected.length !== 1 || !!activeTask} onClick={() => openDialog("rename")}><PencilSimple size={18}/></IconButton>
              <IconButton title="删除" disabled={!hasRemoteFolder || !remoteTarget || remoteSelected.length !== 1 || !!activeTask} onClick={() => openDialog("delete")}><Trash size={18}/></IconButton>
              <IconButton title="刷新设备文件" disabled={!!activeTask} onClick={() => { setRemoteSelected([]); refreshRemote(value => value + 1) }}><ArrowsClockwise size={17}/></IconButton>
            </> : undefined}>
            {!active && <div className="device-empty">
              <DeviceArt/>
              <h3>{scanning ? "正在查找设备" : owner ? "等待设备重连" : "连接你的 Android"}</h3>
              <p>{owner ? "Relaydroid 已保留当前目录。重新连接此设备，或到设备列表选择其他连接。" : "用 USB 数据线连接手机，解锁屏幕，并在手机的 USB 设置里选择「文件传输」。"}</p>
              <div className="empty-buttons">
                <button className="button secondary" disabled={scanning} onClick={() => void rescan(true)}>{scanning ? <CircleNotch className="spin" size={17}/> : <Usb size={17}/>}扫描 USB</button>
                <button className="button secondary" onClick={() => owner ? setPage("devices") : openDialog("wifi")}><Wifi size={17}/>{owner ? "查看设备" : "Wi-Fi 连接"}</button>
              </div>
              {choices.some(device => device.state === "unauthorized") && <p className="device-note">已找到手机。请在手机上允许 USB 调试。</p>}
            </div>}
          </Files>
        </div>
        <div className="transfer-bar">
          {activeTask ? <button className="transfer-summary" onClick={() => setPage("transfers")}><CircleNotch size={18} className="spin"/><span>正在传输 {activeTask.done}/{activeTask.total}</span><span className="running-name">{activeTask.current}</span><ArrowRight size={15}/></button> :
            <span className="transfer-hint"><Info size={16}/>{active ? "选择文件后发送，或把本机文件拖到右侧" : "连接设备后即可双向传输"}</span>}
          <div className="transfer-buttons">
            <button className="button secondary" disabled={!active || !remoteSelected.length || !!activeTask} onClick={() => void transfer("download", remoteSelected)}><ArrowDown size={17}/>保存到电脑</button>
            <button className="button primary" disabled={!hasRemoteFolder || !localSelected.length || !!activeTask} onClick={() => void transfer("upload", localSelected)}><ArrowUp size={17}/>发送到 Android</button>
          </div>
        </div>
      </div>}

      {page === "devices" && <section className="section-card">
        <header><div><h2>附近或已插入的设备</h2><p>USB · MTP 可直接访问手机文件。ADB 需要在手机上授权。</p></div><div className="workspace-actions">
          {machine?.os === "darwin" && <button className="button secondary" disabled={mountBusy || (!active && !mounted) || !!activeTask} onClick={() => void mountInFinder()}>{mountBusy ? "正在处理…" : mounted ? "推出 Finder 挂载" : "挂载到 Finder（只读）"}</button>}
          <button className="button secondary" disabled={scanning || !!activeTask} onClick={() => void rescan(true)}>刷新列表</button>
        </div></header>
        {choices.length ? <ul className="device-list">{choices.map(device => <li key={deviceKey(device)}>
          <span className="pane-icon"><AndroidLogo size={23} weight="duotone"/></span>
          <div><strong>{device.name}</strong><p>{modeLabel(device.kind)} · {device.state === "device" ? "可用" : device.state === "unauthorized" ? "请在手机上允许 USB 调试" : device.state === "offline" ? "设备离线" : "暂无访问权限"}</p></div>
          {device.kind === "adb-wifi" && <button className="button secondary minor" disabled={!!activeTask} onClick={() => void disconnectDevice(device)}>{device.id === scan?.legacy ? "关闭临时 Wi-Fi" : "断开"}</button>}
          <button className="button secondary" disabled={device.state !== "device" || !!activeTask} onClick={() => selectDevice(device)}>浏览文件<ArrowRight size={15}/></button>
        </li>)}</ul> : <div className="section-empty"><Plugs size={38} weight="light"/><strong>{scanning ? "正在查找设备…" : "暂未发现设备"}</strong><button className="text-button" onClick={() => openDialog("help")}>查看连接方法</button></div>}
        {scan && <div className="diagnostics"><span><Usb size={16}/>MTP · {scan.mtp}</span><span><Wifi size={16}/>ADB · {scan.adb}</span></div>}
      </section>}

      {page === "transfers" && <section className="section-card">
        <header><div><h2>传输任务</h2><p>传输按项目计数，失败详情会保留在列表里。</p></div><button className="button secondary" disabled={!tasks.some(task => task.state !== "running")} onClick={() => setTasks(list => list.filter(task => task.state === "running"))}>清除记录</button></header>
        {!tasks.length ? <div className="section-empty"><ArrowsLeftRight size={38} weight="light"/><strong>暂无传输记录</strong><button className="text-button" onClick={() => setPage("files")}>返回文件浏览</button></div> :
          <ul className="task-list">{tasks.map(task => <li key={task.id}>
            <span className="pane-icon">{task.direction === "upload" ? <UploadSimple size={22}/> : <DownloadSimple size={22}/>}</span>
            <div className="task-text"><strong>{task.direction === "upload" ? "发送到 Android" : "保存到电脑"}</strong>
              <p>{task.current || "正在准备"} · {task.done}/{task.total} 个已处理</p>
              {task.state === "running" && <progress max={task.total} value={task.done}/>}
              {task.error && task.state === "error" && <p className="task-error">{task.error}</p>}
            </div>
            {task.state === "running" ? <button className="button secondary" onClick={() => void cancel(task.id)}>停止</button> : <span className="task-status" data-state={task.state}>{task.state === "done" ? <><CheckCircle size={17}/>已完成</> : task.state === "cancelled" ? "已停止" : "有项目失败"}</span>}
          </li>)}</ul>}
      </section>}
      <footer className="statusbar"><span><i data-active={!!active}/>{active ? currentDeviceLabel : "等待设备"}</span><span>USB / Wi-Fi</span></footer>
    </main>

    {dialog && <Modal title={({
      wifi: "连接 Android", help: "连接帮助", settings: "偏好设置", folder: "新建文件夹", rename: "重命名项目", delete: "删除项目",
    })[dialog]} onClose={close} busy={dialogBusy}>
      {dialog === "wifi" && <>
        <div className="usb-callout"><Usb size={21}/><div><strong>用 USB 数据线连接</strong><p>解锁手机，选择「文件传输」。Relaydroid 会自动查找设备。</p></div><button className="text-button" disabled={scanning || dialogBusy} onClick={() => void rescan(true)}>{scanning ? "扫描中…" : "扫描"}</button></div>
        <div className="modal-tabs" aria-label="Wi-Fi 连接方式">
          <button aria-pressed={wifiStep === "pair"} disabled={dialogBusy} onClick={() => changeWiFiStep("pair")}>六位码配对</button>
          <button aria-pressed={wifiStep === "connect"} disabled={dialogBusy} onClick={() => changeWiFiStep("connect")}>连接已配对</button>
          <button aria-pressed={wifiStep === "legacy"} disabled={dialogBusy} onClick={() => changeWiFiStep("legacy")}>旧版 Android</button>
        </div>
        <form onSubmit={e => { e.preventDefault(); void submit() }}>
          {wifiStep === "legacy" ? scan?.legacy ? <p>临时 Wi-Fi 已启用。用完后可关闭手机上的临时无线调试。</p> : <>
            <p>Android 10 或更早版本可先用 USB 连接并授权，然后切到同一个 Wi-Fi。{usb && <>当前 USB 设备 <strong>{usb.name}</strong>。</>}</p>
            <label className="legacy-consent"><input type="checkbox" checked={legacyConsent} onChange={e => setLegacyConsent(e.target.checked)}/><span>我在可信局域网。此模式的 ADB 数据未加密，用完请关闭。</span></label>
          </> : <>
            <p>让两台设备处于同一 Wi-Fi，在 Android 11 或更新版本的开发者选项中打开「无线调试」。</p>
            <div className="discovery-row">
              <button type="button" className="text-button" disabled={discovering || dialogBusy} onClick={() => void discoverWiFi()}>{discovering ? "正在发现…" : "发现附近设备"}</button>
              {suggested.length > 0 && <select aria-label="发现的无线设备" value={compatibleWiFiService} onChange={e => setEndpoint(e.target.value)}>
                <option value="">选择发现的地址</option>
                {suggested.map(service => <option key={service.name + service.endpoint} value={service.endpoint}>{service.name} · {service.endpoint}</option>)}
              </select>}
            </div>
            <label>{wifiStep === "pair" ? "配对 IP 和端口" : "连接 IP 和端口"}<input required value={endpoint} onChange={e => setEndpoint(e.target.value)} placeholder="192.168.1.20:37099" autoCapitalize="none" autoCorrect="off" spellCheck={false}/></label>
            <p className="field-hint">{wifiStep === "pair" ? "在手机上打开「使用配对码配对设备」，保持该页面开启。" : "填写「无线调试」主页面的连接地址和端口。"}</p>
            {wifiStep === "pair" && <label>配对码<input required pattern="[0-9]{6}" maxLength={6} inputMode="numeric" type="password" autoComplete="off" value={code} onChange={e => setCode(e.target.value)} placeholder="六位数配对码"/></label>}
          </>}
          {formError && <p className="form-error" role="alert">{formError}</p>}
          {scan && scan.adb !== "可用" && <button type="button" className="text-button" onClick={() => external("platform-tools")}>获取 Android Platform-Tools <LinkSimple size={14}/></button>}
          <div className="modal-actions"><button type="button" className="button secondary" disabled={dialogBusy} onClick={close}>关闭</button><button className="button primary" disabled={dialogBusy || (wifiStep === "legacy" && !scan?.legacy && (!usb || !legacyConsent))}>{dialogBusy ? "正在处理…" : wifiStep === "pair" ? "配对" : wifiStep === "connect" ? "连接" : scan?.legacy ? "关闭临时 Wi-Fi" : "切换到临时 Wi-Fi"}</button></div>
        </form>
      </>}
      {dialog === "help" && <div className="help-content">
        <h3><Usb size={18}/>USB 文件传输</h3><p>使用支持数据传输的 USB 线。解锁手机，点击 USB 通知并选择「文件传输」。如果设备无法打开，关闭其他 Android 文件传输应用后重试。</p>
        <h3><AndroidLogo size={18}/>USB 调试</h3><p>在开发者选项中打开 USB 调试，插入数据线后，在手机上授权这台电脑。</p>
        <h3><Wifi size={18}/>Wi-Fi 连接</h3><p>Android 11 及以上可通过「无线调试」用六位码配对。Android 10 及更早版本可用 USB 授权，切换到临时 Wi-Fi。两台设备须处于同一个 Wi-Fi。</p>
        <button className="text-button" onClick={() => external("platform-tools")}>Android SDK Platform-Tools <LinkSimple size={14}/></button>
      </div>}
      {dialog === "settings" && <div className="settings">
        <label className="setting-line"><div><strong>显示隐藏文件</strong><p>显示名称以点开头的文件和文件夹。</p></div><input type="checkbox" checked={showHidden} onChange={e => { setShowHidden(e.target.checked); localStorage.setItem("relay.hidden", e.target.checked ? "yes" : "no"); setLocalSelected([]); setRemoteSelected([]) }}/></label>
      </div>}
      {(dialog === "folder" || dialog === "rename" || dialog === "delete") && <form onSubmit={e => { e.preventDefault(); void submit() }}>
        {dialog === "delete" ? <p>从 Android 删除「{remoteTarget?.name}」{remoteTarget?.directory ? "及其中的内容" : ""}？此操作无法撤销。</p> :
          <label>{dialog === "folder" ? "文件夹名称" : "新名称"}<input required autoFocus value={name} onChange={e => setName(e.target.value)}/></label>}
        {formError && <p className="form-error" role="alert">{formError}</p>}
        <div className="modal-actions"><button type="button" className="button secondary" disabled={dialogBusy} onClick={close}>取消</button><button className={"button " + (dialog === "delete" ? "danger" : "primary")} disabled={dialogBusy}>{dialogBusy ? "正在处理…" : dialog === "delete" ? "删除" : "确认"}</button></div>
      </form>}
      <NoticeStack notices={notices} dismiss={dismiss}/>
    </Modal>}
    {!dialog && <NoticeStack notices={notices} dismiss={dismiss}/>}
  </div>
}

createRoot(document.getElementById("root")!).render(<App/>)
