# 技术方案索引

对应 [../roadmap.md](../roadmap.md) 各阶段，按依赖顺序实现。每份 spec 自带验收标准，交给执行 agent 前无需额外上下文（但 agent 必须先读根目录 `CONTEXT.md`）。

| Spec | 内容 | 依赖 | roadmap 阶段 |
|------|------|------|------|
| [01-extension-reliability.md](01-extension-reliability.md) | 扩展连接自愈、WS 认证、错误码、结果补发、任务队列 | 无 | 阶段 1 |
| [02-multi-browser-routing.md](02-multi-browser-routing.md) | 平台标签、登录态路由、故障切换 | 01（register/错误码） | 阶段 1 |
| [03-cache-and-coalescing.md](03-cache-and-coalescing.md) | 服务端缓存 TTL 体系、max_age/refresh、singleflight、容量清理 | 01（错误码） | 阶段 2 |
| [04-records-and-incremental.md](04-records-and-incremental.md) | `since` 增量标准化、`/api/records` 统一出口、导出 | 无（与 03 并行） | 阶段 2 |
| [05-new-adapters.md](05-new-adapters.md) | YouTube、B 站、Reddit saved、小红书收藏（平台范围已定，不再加其他） | 01–04（错误码/缓存分类/增量/records 映射） | 阶段 4 |

固定原则（见 `CONTEXT.md`，实现中不得违反）：原子能力提供者而非爬虫；每平台独立适配器/API/内容表；不做风控机制；缓存只在服务端；评论、字幕等大体量低时效数据实时返回不落库。
