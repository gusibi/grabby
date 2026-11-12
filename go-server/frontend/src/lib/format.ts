export const formatTimeAgo = (timeStr?: string, language: "zh" | "en" = "en") => {
  if (!timeStr) return language === "zh" ? "未知时间" : "Unknown";
  const date = new Date(timeStr);
  if (Number.isNaN(date.getTime())) return language === "zh" ? "未知时间" : "Unknown";
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  const zh = language === "zh";

  const units: [number, string, string][] = [
    [31536000, "年前", "y ago"],
    [2592000, "个月前", "mo ago"],
    [86400, "天前", "d ago"],
    [3600, "小时前", "h ago"],
    [60, "分钟前", "m ago"],
  ];
  for (const [size, zhLabel, enLabel] of units) {
    if (seconds >= size) return `${Math.floor(seconds / size)} ${zh ? zhLabel : enLabel}`;
  }
  return zh ? "刚刚" : "just now";
};

/** Remove HTML tags and decode the handful of entities that show up in feeds. */
export function stripHtml(input?: string | null): string {
  if (!input) return "";
  return input
    .replace(/<script[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?<\/style>/gi, "")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&hellip;/g, "…")
    .replace(/\s+/g, " ")
    .trim();
}
