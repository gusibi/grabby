import type { Copy } from "@/lib/copy";

const zh = {
  title: "管理授权",
  subtitle: "输入管理员密钥后可使用设置和生成控制。",
  label: "管理密钥",
  placeholder: "输入密钥",
  submit: "登录",
  errInvalidKey: "密钥不正确",
  errNetwork: "登录失败，请检查服务状态",
};

type AuthCopy = typeof zh;

const en: AuthCopy = {
  title: "Admin sign-in",
  subtitle: "Enter the admin key to unlock settings and generation controls.",
  label: "Admin key",
  placeholder: "Enter key",
  submit: "Sign in",
  errInvalidKey: "Incorrect key",
  errNetwork: "Sign-in failed — check that the service is running.",
};

export const authCopy: Copy<AuthCopy> = { zh, en };
export type { AuthCopy };
