export const getCategoryColor = (cat: string) => {
  switch (cat) {
    case "tweet": return "bg-sky-500/10 text-sky-600 dark:text-sky-400";
    case "project": return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
    case "paper": return "bg-purple-500/10 text-purple-600 dark:text-purple-400";
    case "article": return "bg-amber-500/10 text-amber-600 dark:text-amber-400";
    default: return "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400";
  }
};

export const getCategoryLabel = (cat: string) => {
  switch (cat) {
    case "tweet": return "推特 Tweet";
    case "project": return "项目 Project";
    case "paper": return "论文 Paper";
    case "article": return "文章 Article";
    default: return cat;
  }
};
