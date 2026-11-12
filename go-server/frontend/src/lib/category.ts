export const getCategoryColor = (cat: string) => {
  switch (cat) {
    case "tweet": return "bg-sky-500/10 text-sky-600 dark:text-sky-400";
    case "project": return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
    case "paper": return "bg-purple-500/10 text-purple-600 dark:text-purple-400";
    case "article": return "bg-amber-500/10 text-amber-600 dark:text-amber-400";
    default: return "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400";
  }
};

export const getCategoryLabel = (cat: string, language: "zh" | "en" = "zh") => {
  const zh: Record<string, string> = {
    tweet: "推文",
    project: "项目",
    paper: "论文",
    article: "文章",
  };
  const en: Record<string, string> = {
    tweet: "Tweet",
    project: "Project",
    paper: "Paper",
    article: "Article",
  };
  return (language === "zh" ? zh : en)[cat] || cat;
};
