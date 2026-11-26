import { Call } from "@wailsio/runtime"

export type Device = { id: string; kind: "mtp" | "adb-usb" | "adb-wifi"; name: string; state: string; root: string; transport?: number }
export type Entry = { name: string; path: string; directory: boolean; size: number; modified: number; link?: boolean }
export type Location = { name: string; path: string }
export type Machine = { os: string; home: string; places: Location[] }
export type Scan = { devices: Device[]; adb: string; mtp: string; legacy?: string }
export type WiFiService = { name: string; endpoint: string; pairing: boolean }
export type PairResult = { device?: Device; endpoint?: string }
export type Direction = "upload" | "download"
export type TransferRequest = { id: string; device: Device; direction: Direction; destination: string; sources: string[] }
export type Progress = { id: string; current: string; done: number; total: number }

const invoke = <T,>(name: string, ...args: unknown[]): Promise<T> => Call.ByName("main.Relaydroid." + name, ...args)

export const api = {
  machine: () => invoke<Machine>("Machine"),
  local: (path: string) => invoke<Entry[]>("Local", path),
  parent: (path: string) => invoke<string>("Parent", path),
  scan: () => invoke<Scan>("Scan"),
  remote: (device: Device, path: string) => invoke<Entry[]>("Remote", device, path),
  choose: (directory: string, folders: boolean, multiple: boolean) => invoke<string[]>("Choose", directory, folders, multiple),
  reveal: (path: string) => invoke<void>("Reveal", path),
  openURL: (target: string) => invoke<void>("OpenURL", target),
  wifiServices: () => invoke<WiFiService[]>("WiFiServices"),
  pair: (endpoint: string, code: string) => invoke<PairResult>("Pair", endpoint, code),
  connect: (endpoint: string) => invoke<Device>("Connect", endpoint),
  startLegacy: (device: Device) => invoke<Device>("StartLegacyWiFi", device),
  stopLegacy: () => invoke<void>("StopLegacyWiFi"),
  disconnect: (id: string) => invoke<void>("Disconnect", id),
  mkdir: (device: Device, path: string, name: string) => invoke<void>("NewFolder", device, path, name),
  rename: (device: Device, path: string, name: string) => invoke<void>("Rename", device, path, name),
  remove: (device: Device, path: string) => invoke<void>("Remove", device, path),
  transfer: (request: TransferRequest) => invoke<void>("Transfer", request),
  finderMount: (device: Device) => invoke<string>("FinderMount", device),
  finderUnmount: () => invoke<void>("FinderUnmount"),
  cancel: (id: string) => invoke<void>("Cancel", id),
}

export function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

export const deviceKey = (device: Device) => device.kind + "/" + device.id
export const modeLabel = (kind: Device["kind"]) => ({ mtp: "USB · MTP", "adb-usb": "USB · ADB", "adb-wifi": "Wi-Fi · ADB" }[kind])
