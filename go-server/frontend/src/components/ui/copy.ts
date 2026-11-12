import type { Copy } from "@/lib/copy";

/** Copy for the shared UI primitives (buttons, toasts, confirm dialogs). */
const zh = {
  close: "关闭",
  cancel: "取消",
  confirm: "确认",
  scoreLabel: (v: number | string) => `评分 ${v} / 10`,
};

export type UiCopy = typeof zh;

const en: UiCopy = {
  close: "Close",
  cancel: "Cancel",
  confirm: "Confirm",
  scoreLabel: (v: number | string) => `Score ${v} / 10`,
};

export const uiCopy: Copy<UiCopy> = { zh, en };
