import type { Copy } from "@/lib/copy";

const zh = {
  kicker: "System · 设备",
  title: "设备与连接",
  description: "管理连接到 Grabby 服务的浏览器扩展设备，并查看当前的 WebSocket 连接状态。",
  refresh: "刷新状态",

  statusTitle: "服务状态",
  connectedCount: "连接设备数量",
  commEstablished: "已建立浏览器通信",
  commNone: "暂无活跃连接设备",
  tipTitle: "提示",
  tipBody: "使用 Grabby Web 网页抓取功能时，必须先在浏览器中安装并启用 Chrome 扩展，并将扩展中的 ID 与服务注册 ID 保持一致。",

  listTitle: (n: number) => `已注册的设备列表 (${n})`,
  listDesc: "管理已在服务中登记的浏览器扩展设备，可执行断开连接和封禁操作。",
  emptyTitle: "没有检测到已注册的设备",
  emptyBody: "请在下方表单中注册一个新的设备 ID。",

  badgeBanned: "已封禁",
  badgeOnline: "在线",
  badgeOffline: "离线",
  stateBanned: "已禁止连接",
  stateReady: "实时抓取就绪",
  stateWaiting: "等待连接",

  actKick: "断开",
  actBan: "封禁",
  actUnban: "解封",

  registerTitle: "注册新设备 / 浏览器",
  registerDesc: "在 Grabby 服务中登记一个专用的设备连接标识（Connect ID），只有注册的设备 ID 才可以建立连接并接收网页抓取任务。",
  registerSubmit: "确认注册此设备",
  labelConnectId: "设备连接 ID *",
  placeholderConnectId: "例如：ebb35609-5aef-472a-a4fd-50cbea38d8e4",
  hintConnectId: "可在 Chrome 扩展设置面板中复制此 UUID 标识。",
  labelDeviceName: "设备名称 *",
  placeholderDeviceName: "例如：brave, chrome-mac",
  hintDeviceName: "为此设备指定一个易读的标识名称。",

  errRequired: "请填写所有必填字段。",
  errRegisterFailed: "注册失败",
  errRegisterNetwork: "注册失败，设备ID冲突或网络连接错误。",
  registered: (name: string) => `设备「${name}」注册成功！`,

  confirmKickTitle: "断开设备连接",
  confirmKickDesc: (name: string) => `将强制断开「${name}」的 WebSocket 连接。`,
  kicked: (name: string) => `「${name}」的 WebSocket 连接已断开。`,
  kickFailed: "断开失败",

  confirmBanTitle: "封禁设备",
  confirmBanDesc: (name: string) => `「${name}」将被永久封禁，之后无法再次连接。`,
  banned: (name: string) => `「${name}」已被阻止连接。`,
  banFailed: "封禁失败",

  confirmUnbanTitle: "解封设备",
  confirmUnbanDesc: (name: string) => `解封后「${name}」将可以重新连接。`,
  unbanned: (name: string) => `「${name}」现在可以重新连接。`,
  unbanFailed: "解封失败",

  unknownError: "未知原因",
  networkError: "网络错误",
  networkKick: "断开连接失败。",
  networkBan: "封禁设备失败。",
  networkUnban: "解封设备失败。",
};

type DeviceCopy = typeof zh;

const en: DeviceCopy = {
  kicker: "System · Devices",
  title: "Devices & connections",
  description: "Manage the browser extensions connected to Grabby and inspect the WebSocket status.",
  refresh: "Refresh",

  statusTitle: "Service status",
  connectedCount: "Connected devices",
  commEstablished: "Browser link established",
  commNone: "No active connections",
  tipTitle: "Note",
  tipBody: "Web capture requires the Grabby Chrome extension to be installed and its ID to match the ID registered here.",

  listTitle: (n: number) => `Registered devices (${n})`,
  listDesc: "Devices registered with the service. You can disconnect or ban them here.",
  emptyTitle: "No registered devices found",
  emptyBody: "Register a new device ID using the form below.",

  badgeBanned: "Banned",
  badgeOnline: "Online",
  badgeOffline: "Offline",
  stateBanned: "Connection blocked",
  stateReady: "Ready to capture",
  stateWaiting: "Waiting for connection",

  actKick: "Disconnect",
  actBan: "Ban",
  actUnban: "Unban",

  registerTitle: "Register a device",
  registerDesc: "Register a dedicated connection ID (Connect ID). Only registered IDs can connect and receive capture tasks.",
  registerSubmit: "Register device",
  labelConnectId: "Connection ID *",
  placeholderConnectId: "e.g. ebb35609-5aef-472a-a4fd-50cbea38d8e4",
  hintConnectId: "Copy this UUID from the Chrome extension settings panel.",
  labelDeviceName: "Device name *",
  placeholderDeviceName: "e.g. brave, chrome-mac",
  hintDeviceName: "Give this device a recognisable name.",

  errRequired: "Please fill in all required fields.",
  errRegisterFailed: "Registration failed",
  errRegisterNetwork: "Registration failed — the ID may already be in use, or the network is unavailable.",
  registered: (name: string) => `Device “${name}” registered.`,

  confirmKickTitle: "Disconnect device",
  confirmKickDesc: (name: string) => `This will force-close the WebSocket connection for “${name}”.`,
  kicked: (name: string) => `“${name}” was disconnected.`,
  kickFailed: "Disconnect failed",

  confirmBanTitle: "Ban device",
  confirmBanDesc: (name: string) => `“${name}” will be permanently banned and can no longer connect.`,
  banned: (name: string) => `“${name}” is now blocked.`,
  banFailed: "Ban failed",

  confirmUnbanTitle: "Unban device",
  confirmUnbanDesc: (name: string) => `“${name}” will be able to connect again.`,
  unbanned: (name: string) => `“${name}” can connect again.`,
  unbanFailed: "Unban failed",

  unknownError: "Unknown reason",
  networkError: "Network error",
  networkKick: "Could not disconnect the device.",
  networkBan: "Could not ban the device.",
  networkUnban: "Could not unban the device.",
};

export const deviceCopy: Copy<DeviceCopy> = { zh, en };
export type { DeviceCopy };
