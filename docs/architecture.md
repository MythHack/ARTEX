# ARTEX 当前架构、能力与接线状态

> 源码接线、配置生效、CI 结果与实际部署可用性分别记录。本文用于项目理解、资讯适配与人工审阅。

## 核验快照

| 项目 | 当前记录 |
| --- | --- |
| 仓库 | [MythHack/ARTEX](https://github.com/MythHack/ARTEX) |
| 初始完整源码基线（历史） | main；[675479c881bf27f6ceebfb3347a2319f0f170dff](https://github.com/MythHack/ARTEX/commit/675479c881bf27f6ceebfb3347a2319f0f170dff) |
| 提交时间 | 2026-11-01 00:51:12，Asia/Shanghai |
| 初始完整核验日期 | 2026-11-01，Asia/Shanghai |
| 本轮增量核验 | 2026-11-15，Asia/Shanghai；[当前 HEAD](https://github.com/MythHack/ARTEX/commit/ca3b706669c035bb48398221861de8e7af9b602c)；具体范围见增量核验节 |
| 当前 SHA 的 CI | 未取得匹配的工作流或 commit status 证据；不沿用旧结论 |
| 范围 | 服务装配、核心 Agent/LLM、上下文、双图、证据、MCP/工具、后台服务、报告及相关前端 |
| 方法 | 静态源码与必要调用方核对；读取匹配提交已有的 GitHub CI 记录 |
| 部署状态 | 未取得实际部署的配置、进程或服务日志；运行可用性未验证 |
| 初始基线 CI（历史） | 前端构建、后端 Build/Vet 成功；后端 Test 失败；**非当前 SHA** |
| 维护 | 每日检查默认分支、接线和 CI；有实质变化时更新文档并提交维护 PR |

**已实现**表示找到具体实现；**已接线**表示找到调用或注册；**条件接线**需要开关、绑定或外部依赖；**运行未验证**表示缺少部署证据。库函数存在、设计文档、CI 成功和实际可用性分别记录。

## 2026-11-15 当前 main 增量核验与完整变化领域盘点（Asia/Shanghai）

**本轮核验源码**：[ca3b706669c035bb48398221861de8e7af9b602c](https://github.com/MythHack/ARTEX/commit/ca3b706669c035bb48398221861de8e7af9b602c)（提交时间 2026-11-14 22:22:08 Asia/Shanghai）。相对上次完整基线 `675479c8`，GitHub compare 返回 **59 个提交**；本节为对变更路径的分组静态核验，以下旧基线段落和固定链接保留作为历史证据，**不得将旧 CI 或旧 SHA 的能力断言直接当作本轮重新验证**。未访问生产实例、配置数据库或外部模型服务。

### 增量架构关系（与下方四张历史基线图配合阅读）

当前仍以 Go Server/Engine、Agent、PostgreSQL 双图与本地流量/证据文件为核心。新增或调整的可见数据关系：

- Server 启动装配 `wireAgentAugment`、`wireTools`，恢复持久化任务后运行 Scheduler；Notifier 独立运行，并异步发现空 MCP 工具缓存。MCP 工具 **schema 延迟向模型暴露** 不等于延迟建立连接：`wireAgentAugment` 在 agent run 中对启用且可见/Skill 引用的 MCP 调用 `connectMCP` 和工具列表；Skill 触发后才解锁相应工具。失败连接有十分钟跳过重试机制。[启动](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/server/server.go)、[MCP 装配](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/server/assembly.go)
- LLM 任务路由存在 profile 选择、切换审计和严格额度耗尽判断；**冷图 Compactor 仍使用 Planner runtime**：`NewCompactor(plannerRuntime, "task-router")`，不能写成独立 Compactor 模型已接线。NOA、Norma 内置会话压缩、冷图摘要仍为不同机制；本次未验证外部 norma SDK 的内部执行语义。[任务路由](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/server/task_llm.go)
- 报告路径增加交付状态排除、资产可读名、复测记录和封面信息；**Word 流量证据装配疑点仍存在**：`exportTaskDocx` 从 `ListFindingsForExport` 读取并直接调用模板，`exportFindingDocx` 从 `GetFinding` 直接调用模板；所读代码未见补充 `TrafficBindings` 切片。任务 Word 的 `QueryByType` 资产附录未在此处传入任务范围。不能将 Word 流量证据和资产范围标记为端到端通过。[报告 HTTP 路径](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/server/report_export.go)
- 新增 `task_artifacts` 元数据、可选 finding 关联及文档预览状态；附件的实际文件仍在任务目录，本轮未确认所有导出格式都包含该类附件。Office/PDF 取样预览为单独组件，需调用接线和外部渲染环境支持。[附件表](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/db/task_artifacts.go)、[预览组件](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/docpreview/preview.go)
- `deepSeekTools` 只有在 WebSearch 已开启、backend 为 deepseek 且 API Key 非空时才返回搜索/抓取工具；工具构造存在不等于已绑定到全部角色。[工具](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/agent/deepseek_search.go)
- Jev / System One 存在独立客户端及配置测试入口，Server 显式拒绝将该格式作为聊天 Provider；仅可按专用判断模型路径使用。外部服务可用性未知。[客户端](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/intercept/jev/client.go)、[服务端](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/server/server.go)
- `db/task_auth.go`、`db/task_scope_guard.go`、`db/task_progress.go`、`db/task_no_result.go` 等已增加持久化实现；数据库文件存在不能单独证明所有 API/UI、任务生命周期及开关条件均已接线。[任务鉴权数据](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/db/task_auth.go)、[范围约束数据](https://github.com/MythHack/ARTEX/blob/ca3b706669c035bb48398221861de8e7af9b602c/db/task_scope_guard.go)

### 增量能力与验证矩阵

| 能力/变更 | 实现 | 调用或入口 | 条件及外部依赖 | 当前 CI / 运行 |
| --- | --- | --- | --- | --- |
| Server 后台任务恢复、Scheduler、Notifier | 已见 | Server 启动及恢复路径已接线 | PostgreSQL、持久化状态、通知配置 | 当前 SHA 无可用匹配检查；部署未知 |
| MCP 可见性、Skill 延迟 schema | 已见 | Agent run 装配、连接和解锁已接线 | MCP enabled、角色可见性、外部可执行程序及网络 | 当前 SHA 无可用匹配检查；实际连接未知 |
| 任务 LLM 选择与切换审计 | 已见 | taskLLMRuntime 已接线 | profile、凭据、模型 API、可选模型池 | 当前 SHA 无可用匹配检查；运行未知 |
| 冷图 Compactor | 已见 | 仍复用 Planner runtime | 图压缩阈值、模型、预算 | 当前 SHA 无可用匹配检查；压缩质量未知 |
| Word 导出增强 | 已见 | 两个 HTTP 导出函数已接线 | 数据库、模板、文档渲染 | 当前 SHA 无可用匹配检查；流量绑定/任务资产范围待验证 |
| 任务附件与文档预览 | 已见 | DB 接口及独立预览组件存在；完整入口链本轮未核完 | 本地文件、预览渲染依赖 | CI 与部署均未知 |
| DeepSeek Web Search | 已见 | 条件性工具构造；角色级绑定待核 | enabled、backend、API Key、外部 API | CI 与外部服务均未知 |
| Jev / System One 判断客户端 | 已见 | 服务端测试入口；聊天 Provider 显式拒绝 | 专用判断配置、外部 API | CI 与部署均未知 |
| 任务鉴权、范围约束、进度与无成果数据 | 已见 | DB 实现存在；完整生命周期接线待核 | 数据库迁移、调用入口及开关 | CI 与部署均未知 |

**当前提交的 CI 证据边界：** GitHub 连接器对 `ca3b706669c035bb48398221861de8e7af9b602c` 查询的 PR 触发工作流返回空列表，combined commit statuses 也为空；这些接口不足以证明没有其他类型的 Actions run。当前提交的 Build、Vet、Test、前端构建均标记 **未取得匹配检查证据**，不能沿用 2026-11-01 的旧 CI 成败。没有主动重跑工作流。

**存储与业务模型边界：** PostgreSQL 仍为任务/资产/探索/配置等主数据；SQLite 为流量索引，流量正文、证据、附件、转录和工作区为本地文件。资产图与探索图不自动等于独立业务状态机；录入校验不等于状态更新全过程的不变量。缓存区分上游会话协作、MCP 工具缓存和冷摘要版本复用，不声称存在通用语义响应缓存。未读取部署开关或凭据。

### 全量差异分组核验与产品入口（2026-11-15）

对初始基线至当前 HEAD 的 GitHub compare 结果进行了分组：59 次提交，接口返回 300 个变更路径（**该返回可能截断，不能将 300 当作全仓变更文件总数**）。按 Agent、Server、DB、前端、部署与外围集成检查变更范围；对核心链路读取当前源码的实现与相应入口。测试文件、README 和历史 PR 说明仅作索引，以下状态以所列当前源码为准。**静态核验不等于实际运行测试，也不表示已逐行审计全部变更文件。**

| 变化领域 | 当前实现与实际接线 | 配置/依赖与边界 | CI / 部署 |
| --- | --- | --- | --- |
| Agent 规划、执行与证据输出 | `agent/planner.go`、`worker.go`、`mainagent.go`、`tools.go` 提供各角色执行入口；`agent/tool_outputs.go` 存在工具输出证据核对账本；`server/engine.go` 负责任务运行协调 | 证据核对模式、工具绑定、角色模型及任务状态会影响生效；工具输出校验不证明外部事实真实性 | 当前提交检查未知；实例未验证 |
| 无成果止损、任务进度与约束 | `db/task_no_result.go` 实现阈值检查，`server/task_runtime.go` 计算实际运行时间，`server/server.go` 的任务创建参数包含止损字段，`web/src/lib/api.ts` 序列化对应字段；`db/task_progress.go` 记录进度 | 止损为可选参数；不能从字段存在推断已开启。暂停/排队与累计时间区别需保留 | 当前提交检查未知；实例未验证 |
| 资产作用域与任务鉴权 | `db/task_scope_guard.go` 对任务自有、来源及外部资产分类；`db/task_auth.go` 解析和规范化任务鉴权配置；任务范围和授权条件影响可访问资源 | 数据层分类/解析已实现；不把它等同于所有入口的完整访问控制证明 | 当前提交检查未知；实例未验证 |
| 任务附件与文件交付 | `db/task_artifacts.go` 保存附件元数据；`server/task_files.go` 提供任务文件列表、读取、预览、归档；`server/server.go` 注册 `/api/tasks/{id}/files*`；`web/src/lib/api.ts` 与任务 FilesTab 调用对应 API | 附件记录与工作区文件是不同对象；完整证据包自动装配不能由文件下载 API 推断 | 当前提交检查未知；实例未验证 |
| 结构化数据预览与文档取样 | `server/artifact_data_preview.go` 解析 JSON/JSONL/CSV/文本并处理预览；`docpreview/preview.go` 提供 Office/PDF 取样组件 | 预览、原件下载、报告附件是不同数据流；Office/PDF 渲染可能需要外部程序，实际环境未验证 | 当前提交检查未知；实例未验证 |
| 报告与证据交付 | `server/report_export.go` 的 Word 路径新增状态过滤、资产名、复测、委托方/时间信息；`report/docx_image.go`、`docx_fonts.go` 为图片/字体组件；前端 ReportTab 调用 `downloadTaskDocx` | Word 流量绑定仍未见补齐数据切片；任务资产附录查询未传任务范围。字体/图片处理存在不代表每种文档渲染通过 | 当前提交检查未知；实例未验证 |
| 拦截与外部判断 | `server/intercept.go` 在 Jev profile 分支调用 `intercept/jev/client.go` 并记录判断用量；`server/server.go` 排除 Jev 聊天 Provider；系统 Intercept 页面支持选择及测试 | Jev 仅为专用判断协议，依赖 profile/密钥/网络；不是普通聊天模型 | 当前提交检查未知；外部服务未验证 |
| 搜索与情报 | `server/manager.go` 读取搜索配置、条件启动漏洞情报同步；`server/server.go` 按 Agent 可见性过滤搜索；`agent/chat.go`、`planner.go`、`worker.go`、`mainagent.go` 条件追加 DeepSeek 工具 | DeepSeek backend + API Key + agent WebSearch；情报同步独立受设置和网络影响。同步开关默认值不等于实例已启用 | 当前提交检查未知；外部服务未验证 |
| 网络出口与代理 | `server/proxy_pool.go` 管理代理池状态，`proxy_autoresume.go` 处理故障与恢复，`egress_probe.go` 提供出口检测；系统代理池页面和管理 API 为配置入口 | 代理池、流量捕获 MITM、WebSearch 出口代理分别配置；探测通过不能证明所有 Agent 流量均走同一路径 | 当前提交检查未知；实例未验证 |
| 工作区及任务文件安全 | `server/workspace.go` 的路径解析与保护、`server/task_files.go` 的受限任务根目录文件操作；对应前端 Workspace/FilesTab | 路径检查为代码防护，不能代替实际平台权限与符号链接回归验证 | 当前提交检查未知；实例未验证 |
| Scheduler、Notifier、恢复与运维 | `server/server.go` 中恢复任务、独立 Scheduler/Notifier、MCP 缓存发现及代理监督；`server/report_recovery.go` 存在空报告恢复函数；`cmd/artex-maint/main.go` 提供独立维护 CLI | 后台循环需持久化状态及配置；独立 CLI 存在不表示主服务自动执行其全部操作 | 当前提交检查未知；实例未验证 |
| 费用、计量与记录 | `db/llm_usage.go` 维护调用用量，`server/cost.go` 提供按模型估算费用的 API；LLM Records 页面可切换完整录制 | 估算价格表不能代表实时账单；完整录制开关与轻量用量账本分离 | 当前提交检查未知；实例未验证 |
| Web UI / API | 任务、文件、报告、拦截、LLM、通知、计划、漏洞、代理池等页面均存在 API 客户端调用；对应服务端注册/函数在本轮重点核对 | 页面存在及编译成功（若有）不能代替浏览器端到端交互、权限、部署测试 | 当前提交检查未知；实例未验证 |
| 安装、容器与发布 | `Dockerfile`、`docker/docker-entrypoint.sh`、`install.sh`、`start.sh`、`build.sh` 及 `.github/workflows/release.yml` 在变更范围；`.github/workflows/ci.yml` 定义后端 Build/Vet/Test 和前端静态构建 | 脚本和工作流配置是可执行路径的定义，不证明当前 SHA 通过、镜像已发布或生产部署健康 | 当前提交无匹配检查证据；部署未知 |

**当前 CI 精确口径：** `.github/workflows/ci.yml` 定义 `go build ./...`、`go vet ./...`、`go test -p 1 ./... -count=1` 与 `npm run build:static`；当前 HEAD 的 GitHub PR workflow-run 查询和 combined statuses 均未返回匹配结果。由于连接器仅查询部分事件/页，状态应记为“**本次未取得可核对的当前 SHA run/attempt**”，而不是“没有 CI”或“CI 失败”。没有重跑、没有读取生产部署，也不将旧 attempt 的结果迁移到本次。

**验证分级：** 表中“已实现”=源码可见，“已接线”=可见的调用/路由/UI 链，“条件接线”=需要开关、角色绑定或外部服务；CI 只有相同 SHA、run attempt 的具体 job 结果才能确认；“运行已验证”需要部署实例与对应操作证据，本轮全部保留未知。

**四张 Mermaid 图的阅读约定：** 下方四图分别是整体服务、LLM/上下文、数据/证据和后台服务的高层依赖关系。箭头表示数据或调用关系而非“部署已运行”；图中没有展示的新增模块（如任务附件预览、Jev 判断、代理池）由本节矩阵补充，不能凭图未画出推断不存在。

## 整体架构

箭头表示模块依赖或数据关系，条件连接受配置影响。

```mermaid
flowchart TD
    UI["Next.js Web UI"] --> API["Go HTTP API · 认证 · SSE"]
    API --> APP["Manager · Engine · 会话服务"]
    APP --> AG["Agent 角色与工具装配"]
    AG --> LM["LLM 适配 · 计量 · 可选模型池"]
    AG --> TOOL["内置工具 · Skill · MCP · 审阅"]
    APP --> PG["PostgreSQL：任务、双图、配置与记录"]
    API --> BG["后台调度 · 情报同步 · 通知与维护"]
    BG --> PG
    TOOL -. 条件：流量记录 .-> TR["SQLite 流量索引与正文文件"]
    TOOL --> FS["本地文件：证据、转录、工作区与归档"]
    TR -. 证据快照 .-> FS
    API --> FS
```

入口 `run()` 创建 Manager、Server 和 HTTP 服务；`server.New` 将任务 Agent 解析器接到 Engine，并装配工具和后台服务；`NewManager` 打开 PostgreSQL，按条件创建流量和情报组件。[入口][arch-main]、[Server][arch-server]、[Manager][arch-manager]

### 启动与存储边界

| 层 | 当前接线 | 生效与可用条件 |
| --- | --- | --- |
| 主服务 | `run → NewManager → server.New → Handler`。[入口][arch-main] | Manager 需要 PostgreSQL；部署连通性未验证。 |
| 任务/会话控制 | `server.New` 设置任务级及固定 Worker 解析器、恢复任务、检查 readiness。[Server][arch-server] | 全局模型就绪与单任务能否解析可用模型分别判断。 |
| 工具装配 | `wireAgentAugment / wireTools` 处理 Skill/MCP、角色绑定、开关和 schema。[装配][arch-assembly] | 实际工具集取决于配置与外部依赖。 |
| PostgreSQL | 保存任务、资产/探索关系、配置、审批、用量及业务元数据。[Manager][arch-manager] | 本次未连接实际数据库。 |
| 流量 SQLite | `traffic.Open` 创建 `_index/index.sqlite` 与正文目录。[traffic][arch-traffic] | 流量实例初始化、监听成功和 capture 开关分别判断。 |
| 本地文件 | 保存流量正文、独立证据、会话转录、工作区及归档。[证据][arch-evidence] | 需要权限、磁盘空间和可读原文；实际状态未验证。 |

## 当前 CI 与运行验证

基线提交的 [CI 运行 41221669712][arch-ci-run] 于 2026-11-01 00:54:34（Asia/Shanghai）结束，总结论为 **failure**。这是读取已有记录，未重新执行或部署项目。

| 检查 | 已记录结果 | 结论范围 |
| --- | --- | --- |
| Frontend Install / Build (static export) | 成功 | 该 CI 环境的安装、构建与静态导出通过。 |
| Backend Build | 成功 | `go build ./...` 通过。 |
| Backend Vet | 成功 | `go vet ./...` 通过。 |
| Backend Test | 失败 | 全量回归状态不能记为通过。 |
| 用户部署 | 未验证 | 缺少配置、连通、数据库健康与实际操作证据。 |

失败记录涉及 `agent`、`db`、`intercept`、`selfupdate` 包。顶层失败测试包括 `TestCaptureApprovalLifecycle`、`TestInterceptApprovalFilters`、`TestSideTaskArchiveVersions`、`TestJudgeRetriesBeforeApplyingFailurePolicy`、`TestFetchLatestUsesForkRelease`、`TestFetchLatestMissingRelease`。失败根因及部署影响尚未确认；日志中的预期失败样例或数据库探活错误不能自动视为独立产品故障。[工作流][arch-ci]、[backend job][arch-ci-backend]、[frontend job][arch-ci-frontend]

## LLM、上下文与缓存

本节的代码基线为 [`675479c8`](https://github.com/MythHack/ARTEX/commit/675479c881bf27f6ceebfb3347a2319f0f170dff)。下文“已实现并接线”表示已核对实现及调用位置；开关是否开启、网关是否支持、实际效果是否满足要求，仍须由对应部署的配置和运行记录确认。

### 调用与上下文关系

```mermaid
flowchart TD
    S["Agent 会话"] --> C{"NOA 已开启且接入成功？"}
    C -->|否| B["Norma 内置会话压缩"]
    C -->|是| N["NOA：上下文管理与原文归档"]
    B --> R["任务 LLM 路由"]
    N --> R
    G["冷图摘要：版本签名复用"] -->|需要生成摘要时复用 Planner 运行时| R
    R --> P["选定 Provider；模型池按配置接入"]
    P --> U["调用计量与可选报文录制"]
    U --> A["模型 API"]
```

这张图描述当前任务路径的关系。**NOA 与 Norma 内置会话压缩互斥；冷图摘要是独立的图状态派生功能。** 开启 NOA 并不意味着移除冷图摘要。图中省略了 HTTP 传输层的限流、会话头和重试细节；模型池是否接入取决于配置来源及开关。[会话接线][llm-task]、[NOA 接线][llm-noa]、[冷图摘要][llm-compaction]

### 能力、接线与可用状态

| 能力 | 当前实现及接线证据 | 配置条件与状态 | 运行验证状态 |
| --- | --- | --- | --- |
| 多种模型 API 接入 | `ConfigFrom` 支持 Anthropic、OpenAI Chat Completions、OpenAI Responses；提供流式/非流式、输出上限及思考参数配置。[provider][llm-provider] | **已实现**；依赖有效 profile、凭据及端点兼容性。 | 未连接实际模型验证；配置项存在不代表上游支持。 |
| 任务模型解析 | `taskLLMRuntime.current` 先处理固定模型 Worker，再按角色绑定、任务链、全局配置解析；配置链耗尽会显式报错。[task_llm][llm-task] | **已实现并接线**；具体选择由部署配置决定。 | 未读取实际 profile、角色绑定或任务链。 |
| 模型池与熔断 | `poolChain`、`order`、健康注册表实现显式优先级、同优先级轮换、窗口过滤、失败转移和冷却。[接线][llm-pool-wiring]、[模型池][llm-pool]、[健康状态][llm-health] | **配置可选**；`llm_pool_enabled` 默认关闭；绑定模型回退还有独立开关。 | 未确认部署是否开启。不能把容错选路描述为基于质量/成本预测的自动选模。 |
| Norma 内置会话压缩 | `compactionConfig` 为会话生成窗口配置；任务运行时解析角色对应窗口。[provider][llm-provider]、[task_llm][llm-task] | **已实现并接线**；NOA 未开启或接入失败时保留这条路径。 | 未验证实际触发次数、摘要效果或节省比例。 |
| NOA 上下文管理 | `enableNoa` 调用 `noaadapter.Enable`，接入原文归档；成功后清除 `opts.Compaction`，失败时保留内置会话压缩。[noa][llm-noa] | **实验性、配置可选**；`noa_compaction` 默认关闭，每次 run 读取；不是冷图摘要的替代开关。 | 未确认启用状态，未验证归档完整性和压缩质量。 |
| 冷图摘要与摘要复用 | `coldgraph.go` 计算冷热、关系分组及内容版本签名；`major()` 复用签名未变的摘要，避免再次调用模型；新摘要通过 `compress()` 生成。[图算法][llm-coldgraph]、[压缩实现][llm-compaction] | **已实现并接线**；`agentsForTask` 把 Compactor 注入 Planner。触发依赖图状态和阈值。 | 未验证运行中的摘要命中率、语义保真、净 token 节省。 |
| Compactor 模型来源 | 当前任务构建调用 `NewCompactor(plannerRuntime, "task-router")`，因此这条路径复用 Planner 运行时。[task_llm][llm-task] | **文档与任务实现有差异**：模型路由策略文档称 Compactor 可独立配模，当前任务接线没有建立独立 Compactor 运行时。 | 独立选模的可用性不能据策略文档确认；需要定位实际调用路径。 |
| 压缩输出预算 | 任务构建注入 `CompactionMaxTokens/CompactionThinking`；`resolveBodyBudget` 每次调用读取这些解析器。[manager][llm-manager]、[compaction][llm-compaction] | **已实现并接线**；当前任务路径的未配置默认为 6000、`disabled`。构造器保留的 65536 不是这条路径的有效默认。 | 未读取实际设置；上游对思考参数的行为仍须实测。 |
| Token 与调用记录 | `Recorder.Stream/Complete` 写轻量用量记录；完整请求、响应和原始报文由独立开关控制；计量含任务、角色、模型、profile、耗时、输入/输出与缓存 token。[Recorder][llm-rec]、[账本][llm-usage] | **已实现并接线**；轻量计量不受完整录制开关控制。落库为尽力而为，上游 usage 缺失会限制计量。 | 未核验当前部署的记录完整度、费用换算或不同调用路径的归因。 |
| HTTP 尝试限流 | `RequestLimiter` 在 HTTP 尝试层按秒/分钟滚动窗口控制请求，包括重试。[限流器][llm-limiter] | **已实现**；限额由 profile 配置；不是 token 预算限制。 | 未验证实际配置及流量下的限流行为。 |

### 缓存语义

当前代码中有三种不同含义，架构更新和资讯适配判断应保持区分：

1. **上游缓存协作。** `SessionHeaderKey` 可让请求携带稳定会话 ID，供支持该约定的网关使用。是否缓存及是否命中由上游决定；这本身不是 ARTEX 的语义响应缓存。[provider][llm-provider]
2. **缓存用量计量。** `CacheRead/CacheWrite` 保存供应商返回的缓存 token；有统计字段不能证明主动缓存功能或缓存命中。[Recorder][llm-rec]、[账本][llm-usage]
3. **冷图摘要精确复用。** 摘要签名覆盖成员、锚点的 ID 和内容版本；签名未变时可以直接复用既有摘要。这是已核实的调用减少机制。[coldgraph][llm-coldgraph]、[compaction][llm-compaction]

本轮核对的核心路径没有显示通用的向量相似请求命中后直接复用回答机制，也没有显示基于质量或费用预测的学习型选模器。该结论限定于已核对的 ARTEX 代码路径；外部 Norma SDK 和上游服务的内部实现未在本节逐文件验证。

### “能力不变、用量减少”的证据要求

当前实现能提供 token、缓存 token、调用耗时和错误状态等数据，但这些指标单独不能证明能力保持不变。评价新算法时，应在相同脱敏材料与任务要求下核对事实保留、引用正确性、结论一致性及遗漏，同时计入摘要、重试和缓存维护本身的消耗。需要权重访问、KV cache 控制或自托管推理引擎的方法，应标明额外部署条件；不能描述为现有模型 API 接入方式即可直接使用。本文不声称当前部署已达到某个节省比例。

[llm-provider]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/provider.go
[llm-task]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/task_llm.go
[llm-noa]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/noa.go
[llm-compaction]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/compaction.go
[llm-coldgraph]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/coldgraph.go
[llm-pool-wiring]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/llmpool.go
[llm-pool]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/llmpool/pool.go
[llm-health]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/llmpool/health.go
[llm-manager]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/manager.go
[llm-rec]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/llmrec/llmrec.go
[llm-usage]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/llm_usage.go
[llm-limiter]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/request_limiter.go

## 数据、证据、展示与交付

本节核对基线为提交 `675479c881bf27f6ceebfb3347a2319f0f170dff`。下列“已接线”表示已找到实现、调用者及接口或界面入口；没有启动实例、连接运行数据库或执行端到端验证，因此不表示用户部署已经启用、数据完整或运行效果达标。配置开关与数据库中的工具绑定会继续影响实际可用性。

### 数据关系

```mermaid
flowchart TD
    A["共享资产库与任务探索图"] -->|探索节点| D["冷摘要与成员索引"]
    A --> Q["只读查询与来源校验"]
    S["直接来源任务"] --> Q
    D --> Q
    Q --> V["图与审阅页面"]
    A --> F["发现和任务情报"]
    F --> E["版本化证据快照"]
    E --> B["SHA256 正文存储"]
    F --> C["归并记录与人工复核"]
    C --> X["报告与证据导出"]
    E --> X
```

图中的来源任务通过显式关联提供只读资料；冷摘要保存成员关联，完整记录仍可追溯。证据快照中的元数据保存在 PostgreSQL，请求与响应正文按内容哈希保存在文件系统。不同导出格式的数据装配并不完全相同，不能仅凭共用模板推定各格式都已携带完整流量证据。

### 能力、接线与可用状态

| 能力 | 实际实现 | 调用与接线证据 | 配置或依赖 | 核对状态 |
| --- | --- | --- | --- | --- |
| 资产图、探索图与血缘 | 资产、探索节点、关系边和资产锚点分别存储；探索记录可保留事实、发现、目标及来源。覆盖图从资产库及任务范围构造，探索图读取节点与边。 | [db/assets.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/assets.go)、[db/exploration.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/exploration.go)；[server/server.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go) 的 `taskCoverageGraph` 调用 `BuildCoverageGraph`，`explorationGraph` 读取 `Nodes/Edges`，对应路由已注册。 | PostgreSQL、已存在的任务与资产数据。覆盖度展示受任务 `coverage_enabled` 影响。 | 数据与 HTTP 读取路径已接线；真实数据规模下的完整性、查询性能及运行状态未知。 |
| 冷摘要、成员展开与原文恢复 | `digest` 节点通过 `covers` 边记录成员；摘要有有效/被替代状态，原成员继续保留；`expand_digest` 返回成员紧凑信息，完整详情另行读取。 | [db/digest.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/digest.go) 的 `AddDigest/DigestMembers/SupersedeDigests`；[agent/compaction.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/compaction.go) 调用 `AddDigest`；[server/task_llm.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/task_llm.go) 的 `agentsForTask` 创建并接入 Compactor；[agent/tools_digest.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/tools_digest.go) 与 [agent/tools.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/tools.go) 注册展开工具。 | 可用的模型通道、压缩阈值和预算、符合压缩条件的数据；实际工具可见性受数据库绑定影响。 | 生成、存储、展开路径已接线；不等同于摘要质量已验证，也不等同于已实现语义检索。 |
| 直接关联任务的只读继承 | `task_relations` 指明来源；节点和活动带 `inherited/source_task_id`；只展开直接关系，不递归读取来源的来源。继承意图仅暴露允许的终态。 | [db/exploration_sources.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/exploration_sources.go) 的 `DirectSourceStores/GetNodeWithSources`；[server/server.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go) 的 `explorationGraph` 合并经 `inheritedGraphSnapshot` 过滤的来源图；[server/finding_traffic.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/finding_traffic.go) 的 `findingTrafficAccess` 与 [server/finding_cases.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/finding_cases.go) 的 `findingCaseAccess` 拒绝在继承上下文写入。 | 已保存的来源任务关系、未删除的来源任务；来源任务的修改应回到原任务上下文。 | 查询与上下文写保护已接线；部署中的关系配置及实际资料覆盖未知。 |
| 任务情报的事实与推断区分 | 记录类型、来源证据、资产、状态、`observed/inferred`；录入校验拒绝将推断直接标为验证成功；内容指纹去重，系统绑定任务与来源节点。可随事实批量写回。 | [db/intelligence.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/intelligence.go) 的 `NormalizeIntelligence/RecordIntelligence`；[agent/intelligence.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/agent/intelligence.go) 的 `intelligenceWorkflow`；[server/assembly.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/assembly.go) 的 `wireTools` 注入记录回调；[server/intelligence.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/intelligence.go)、[intelligence-panel.tsx](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/components/intelligence-panel.tsx) 接入读取与状态编辑。 | `task_intelligence_enabled` 在读取缺省值时为 `false`；还需当前任务上下文及工具绑定。 | 条件接线已确认；不能据源码断言部署已开启。关键词检索主要使用 SQL `ILIKE`。 |
| SHA256 正文与证据快照 | 请求/响应正文内容寻址、长度和哈希校验、相同内容复用；快照与可清理的原始流量分离；绑定变化递增证据版本；无引用数据按保留窗口回收。 | [evidence/store.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/evidence/store.go) 的 `writeBody/Record/Bind/OpenBody/RunGC`；[db/finding_traffic.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/finding_traffic.go) 的快照、绑定及版本函数；[server/task_llm.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/task_llm.go) 将证据 Store 注入各任务 Agent；[server/server.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go) 启动证据 GC。 | PostgreSQL、可写证据目录；新流量绑定需要原始流量记录存在且正文可读取。 | 持久化、读取与 GC 调用已接线；文件系统、数据库和现有证据的运行健康未知。 |
| 证据审阅、分页读取与版本约束 | HTTP 接口支持绑定、用途/备注编辑、排序、删除、详情、正文分段读取与下载；写入使用版本校验；保存报告时记录所依据的证据版本。 | [server/finding_traffic.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/finding_traffic.go) 的 `registerFindingTraffic` 注册接口；[server/orchestration.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/orchestration.go) 注册 `get_finding_traffic/bind_finding_traffic`；[finding-traffic-panel.tsx](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/components/finding-traffic-panel.tsx) 经 [web/src/lib/api.ts](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/lib/api.ts) 调用；[server/finding_workflow.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/finding_workflow.go) 校验绑定工具开关。 | Agent 自动绑定受 `agent_traffic_binding` 控制，缺省读取为 `false`；已保存证据的读取不依赖捕获开关。 | REST、工具和界面读取路径已接线；实际工具授权、捕获状态和证据数量未知。 |
| 精确去重、归并建议与人工复核 | 同任务的符合条件记录按类别、名称和资产指纹合并追加证据；归并组保存成员与事件，疑似分组可人工接受/否决并保留否决约束；统一报告使用组版本校验。 | [db/finding_traffic.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/finding_traffic.go) 的 `FindingFingerprint/RecordFindingTx`；[db/finding_cases.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/finding_cases.go) 的归并函数；[server/finding_cases.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/finding_cases.go) 注册工具并提供审核入口；[finding-case-list.tsx](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/components/finding-case-list.tsx) 调用建议、接受/否决、成员查询接口。 | 人工发起的模型整理需 Reporter 启用且模型可用；手动操作与模型工具仍受对应范围和绑定约束。 | 数据、HTTP、工具及界面已接线。疑似分组需要人工确认；证据充分时 Reporter 存在直接归并入口，不能概括为“所有归并逐条等待人工批准”。 |
| Markdown、CSV、JSON 与证据 ZIP | 支持按筛选、所选记录或全部记录导出；可按归并组整合；导出前冻结证据绑定/报告版本，ZIP 复制证据正文并包含可引用附件。 | [server/server.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go) 的 `findingsExport` 调用 `StageFindingsExport`，再调用 [report/findings.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/report/findings.go)；[server/finding_cases_export.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/finding_cases_export.go) 处理整合；[web/src/lib/api.ts](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/lib/api.ts) 的 `exportFindings` 触发下载。 | PostgreSQL、可读证据正文、可写临时目录、足够存储空间。 | 已找到端到端静态调用链；未验证下载、文件内容与大体积导出。 |
| DOCX 总报告与单条报告 | Word 模板、HTTP 导出入口及前端下载调用存在；可处理报告正文、资产名称、严重度分组和复测记录。模板还包含流量证据小节。 | [server/report_export.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/report_export.go) 的 `exportTaskDocx/exportFindingDocx` 调用 [report/docx_build.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/report/docx_build.go) 的模板；任务 [report-tab.tsx](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/app/%28main%29/function/tasks/detail/_tabs/report-tab.tsx) 经 `downloadTaskDocx` 调用。 | PostgreSQL；读取已保存报告不要求新的模型调用。 | 基本导出路径已接线；**流量绑定到 Word 的数据装配存在待核对项**，详见下文，不能记为完整可用。未进行 Word 渲染验证。 |
| 探索图、资产图与网络分区视图 | 探索图具有摘要收缩、关系边去重、成员详情、分层布局及可见元素渲染；覆盖图支持资产/网络两种视图、分区、折叠展开、路径高亮。 | [graph-tab.tsx](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/app/%28main%29/function/tasks/detail/_tabs/graph-tab.tsx) 调用 `api.explorationGraph`；[exploration-graph.tsx](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/components/exploration-graph.tsx) 渲染；[coverage-graph-tab.tsx](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/web/src/app/%28main%29/function/tasks/detail/_tabs/coverage-graph-tab.tsx) 调用 `api.taskCoverageGraph`，后端路由和数据构造函数已对应。 | 前端构建、浏览器、后端 API 和资产/探索数据；关闭覆盖度时仍可显示资产关系，但抹平已测标记。 | UI 与 API 已接线；未做浏览器实测。网络视图的边表示归属/关联，不证明实际网络可达性。 |

### 已确认边界与仍需核对的接线

- **两个图与业务模型的边界。** 已核对的存储和图组件围绕资产、探索血缘、摘要和覆盖展示；这些实现不能作为“独立的模块—业务对象—操作—状态模型已经完成”的证据。业务事实可以存在于原始记录中，但结构化程度和完整性需结合实际数据核验。
- **查询与语义检索的边界。** 所读节点、活动与情报查询主要使用关键词过滤；冷摘要及图结构本身不证明嵌入检索、语义缓存或 GraphRAG 已实现。
- **归并判定的边界。** `FindingCaseCandidates` 使用类别、名称、资产重叠召回候选。候选分数不是根因相同的证明；疑似建议、人工决定与 Reporter 的直接归并入口应分别描述。
- **情报状态校验的边界。** `NormalizeIntelligence` 在录入时限制 `inferred` 与已验证状态组合；人工编辑走的 `UpdateIntelligenceStatus` 单独校验类别和状态，没有再次校验该组合。因此应描述为“录入约束”，不能把它宣称为全生命周期不变量。
- **Word 流量证据接线。** `writeTrafficEvidenceDocx` 仅在 `DBFinding.TrafficBindings` 非空时输出流量小节。但当前 `exportTaskDocx/exportFindingDocx` 分别从 `ListFindingsForExport/GetFinding` 取数据后直接调用模板；这两个查询填充版本及数量，没有填充绑定切片，也没有经过发现页导出的 `StageFindingsExport`。因此只能确认模板存在，不能将 HTTP Word 导出记为完整携带流量绑定。该判断来自上述函数间的静态数据流，尚未运行复现。
- **Word 资产统计范围。** 当前任务 Word 路径调用 `Assets().QueryByType` 构造资产计数及附录，没有在这处传入任务范围。因而不能将该统计描述为已经严格限定于当前任务；实际输出需结合数据库内容核对。
- **运行状态。** 尚未读取实例配置、工具绑定或运行数据库，未验证数据迁移、证据目录权限、开关值、模型可用性、页面渲染及导出结果。后续维护应继续区分“实现存在”“调用已接线”“配置已启用”“实例实测通过”，不要互相替代。

## 外围集成与后台服务

核对基线：`MythHack/ARTEX` 的提交 [`675479c881bf27f6ceebfb3347a2319f0f170dff`](https://github.com/MythHack/ARTEX/commit/675479c881bf27f6ceebfb3347a2319f0f170dff)，提交时间为 2026-11-01 00:51:12（北京时间）。本节是源码静态核对：**实现存在、入口已接线、配置满足、运行成功是四种不同证据**。没有读取部署数据库、连接外部服务或执行项目，因此本节所有外部集成的运行状态均为“未验证”，不能据源码默认值判定用户部署已启用。

### 后台接线概览

```mermaid
flowchart TD
    BOOT["Server 初始化"] --> RESTORE["恢复持久化任务"]
    BOOT --> N["独立 Notifier 循环"]
    RESTORE --> S["独立 Scheduler 循环"]
    PG["PostgreSQL：计划、水位、事件与投递"] --> S
    PG --> N
    S --> INV["计划和事件回调入口"]
    EVT["业务记录及状态变化"] --> PG
    N --> CH["已启用的通知渠道适配器"]
```

图中各入口已在源码中找到；配置是否满足及实际触发、送达状态仍待部署证据。

### 实际接线与依赖

| 能力 | 实现存在 | 入口与实际接线 | 配置和环境依赖 | 本次可用状态 |
|---|---|---|---|---|
| MCP 三类传输、工具发现 | `connectMCP` 分派 stdio、Streamable HTTP、旧 SSE；发现工具名称和描述后写入数据库缓存 | `Server.New` 调用 `wireAgentAugment`，并异步调用 `discoverEmptyMCPsOnStartup`；装配器连接启用且对当前 Agent 可见或被可见 Skill 引用的 MCP，再调用 `Tools` | MCP 行启用、Agent/Skill 可见性、可执行命令或服务 URL、必要认证、对应服务健康 | 源码已接线；配置未核对；连接与工具调用未验证。缓存里有工具名称不代表现在可连接。见 [mcpdiscover.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/mcpdiscover.go)、[assembly.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/assembly.go)、[client.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/mcphttp/client.go)。 |
| Skill 与工具的渐进暴露 | 装配器区分直接可见和 Skill 关联工具；建立调用解锁集合；延迟工具通过统一入口暴露 | `Server.New → wireAgentAugment → agent.ToolAugment`，同时从数据库加载可见 Skill 与工具绑定；`s.hostTools` 合并平台工具、用户自定义工具与条件可用的流量工具 | Skill 目录可读、合法元数据、数据库可见性/绑定、对应工具实现及环境 | 已见调用接线；未验证任何实际 Skill 或工具运行。延迟 schema/解锁不等于完全按需启动外部进程。见 [assembly.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/assembly.go)、[orchestration.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/orchestration.go)。 |
| MCP 失败冷却与输出体积管理 | 按服务名记录连接失败窗口；HTTP/SSE 工具结果交给 SDK `Capture` 处理 | `wireAgentAugment` 实际调用 `mcpFailedUntil / markMCPFailed / clearMCPFailure`；`Client.wrap` 调用 `actool.Capture` | 当前失败状态、SDK 的工具输出目录与输出预算；磁盘可写 | 通用机制已接线；收益和信息保留效果未测。元数据缓存、失败缓存与调用结果缓存应分别表述。见 [mcp_failure_cache.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/mcp_failure_cache.go)、[client.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/mcphttp/client.go)。 |
| 宿主工具与 Python 库清单 | CLI 查 PATH；Python 库批量检查 `importlib.util.find_spec`；字典检查真实目录；有探测缓存与失效入口 | `Server.New` 注入 Python 和数据目录 resolver；`hostenvStatus` 返回 `hosttools.Inventory()`；相关摘要可供系统能力展示使用 | 宿主 PATH、选定 Python、实际安装库与目录、文件权限 | 探测实现与状态 API 已接线；本次未运行 Inventory，不能报告某个工具已安装。见 [hosttools.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/hosttools/hosttools.go)、[hostenv_api.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/hostenv_api.go)、[server.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)。 |
| 可选宿主安装助手 | `hostd` 有白名单、安装任务状态、日志及串行执行；主服务经 Unix socket 转发 | `Server.Handler` 注册宿主状态/安装/任务接口；`hostenvInstall → hostdCall`；`hostd.Server.Start` 再核对目录白名单；`Server.New` 启动助手版本协调 | 单独运行的 `artex-hostd`、可访问的 socket、支持的系统/包管理器、下载连通性和权限；并非所有部署形态都有助手 | 调用路径已存在；助手是否安装、在线、版本一致、安装是否成功均未验证。主服务可运行不代表助手可用。见 [hostenv_api.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/hostenv_api.go)、[hostd/server.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/hostd/server.go)。 |
| 流量记录、检索和大正文存储 | `traffic.Open` 初始化代理、SQLite 索引、内容寻址 blob 和 CA 目录；支持检索、正文读取及索引回收逻辑 | `NewManager` 在非空代理地址下调用 `traffic.Open` 并启动 `Traffic.Start`；`Manager.HostTools` 在记录实例存在且捕获开关打开时返回工具；HTTP 查询接口在 `Server.Handler` 注册；浏览器配置由 `syncBrowserMCPProxy` 协调 | 记录实例初始化成功、代理监听成功、捕获开关、流量实际经过记录链路、CA/网络配置、可写数据目录；全文索引还取决于初始化结果 | 已接线；监听/捕获/检索均未运行验证。对象存在与记录开关为真不等于代理已成功监听；某些协议错误可能透明转发而不留存正文。见 [manager.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/manager.go)、[traffic.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/traffic/traffic.go)。 |
| 公开情报同步与检索 | NVD/KEV、GHSA 补充、可选公开资料聚合；有数据库 Store、时间/分页游标、共享 API 放行控制和来源状态 | `NewManager` 创建 Store 并注入 Agent；启动设置允许时拉起主同步与独立近期更新循环；主同步调用 `vulnintel.Run`，GHSA 由独立更新路径调用；状态、配置和手动同步接口已注册 | PostgreSQL 表、同步开关、外部来源可达性、代理、可选 API 凭据及来源额度；首次补全完成情况 | 实现和条件入口已确认；外部来源当前可达性、同步水位与实际数据完整性未验证。已有同步器不等于库已完整。见 [manager.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/manager.go)、[vulnintel_sync.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/vulnintel_sync.go)、[sync.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/vulnintel/sync.go)、[intel.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/vulnintel/intel.go)。 |
| 规则审查、模型研判和人工审批 | `Guard` 注册工具前后 hook；拦截器有规则匹配、研判、人工等待、审计及取消/超时处理；审查输入含有界人工背景与脚本快照 | 任务恢复/创建使用 `guard.NewWithInterceptor`；对话使用 `chatGuard`；`Server.New → wireInterceptReviewer → Interceptor.SetReviewer`；管理与人工处理接口在 Handler 注册 | 纳管工具配置、规则、研判开关/模型 profile、数据库、审阅者；审查覆盖还取决于具体调用路径 | 代码接线已确认；部署覆盖范围、判定质量与人工流程未验证。已接线不能推导每次调用都经过同一种研判。见 [guard.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/guard/guard.go)、[intercept.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/intercept.go)、[review_context.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/intercept/review_context.go)、[intercept/intercept.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/intercept/intercept.go)。 |
| 计划窗口与事件触发 | Scheduler 读取计划/触发器和持久化水位；支持定时、记录新增、状态变化及工具活动等配置驱动触发；日历窗口记录控制历史 | `Server.New → restoreTaskRuntimes → newScheduler(s).Run`；`Scheduler.step` 调用计划窗口计算和各类触发检查；满足条件时调用现有会话入口；计划和触发器 API 已注册 | PostgreSQL、启用的计划/触发器、目标 Agent 配置及所需模型、时区、任务状态 | 后台循环接线已确认；真实计划触发和后续会话未验证。水位持久化不代表待执行触发队列全部持久化，`triggerQ` 是内存结构。见 [scheduler.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/scheduler.go)、[calendar_schedule.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/calendar_schedule.go)、[server.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)。 |
| 通知事件、投递与渠道 | 记录创建/状态变化可登记通知事件；数据库分派渠道投递；有实时/汇总、领取租约、按渠道限流、退避和分段续发；注册钉钉、飞书、企业微信、Webhook、Telegram、Email 适配器 | `RecordFindingTx / SetFindingStatusTx → RecordNotificationEventTx`；`Server.New` 单独启动 `newNotifier(s).Run`；`Notifier.step → FanOutPendingEvents → Claim… → channel.Send`；管理/测试/历史 API 已注册 | 通知总开关、启用且过滤匹配的渠道、合法目标与凭据、网络可达、待投递事件；详情回链另需外部基础 URL | 生产者到投递者的源码接线已确认；未发送消息；任何渠道的实际可达与送达均未验证。适配器已注册不等于用户已配置。见 [finding_traffic.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/finding_traffic.go)、[notification.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/db/notification.go)、[notifier.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/notifier.go)、[channel.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/notify/channel.go)。 |

### 数据边界应在总架构图中明确

- **PostgreSQL** 保存业务状态、设置、计划、Agent/MCP 元数据、审查记录、通知事件与投递状态等。主要初始化入口为 `server.NewManager → db.Open`。
- **流量 SQLite** 是独立存储：`traffic.Open` 打开 `data/traffic/_index/index.sqlite`；请求/响应元信息和较小正文保存在索引库，较大正文按 SHA256 写入 `data/traffic/_blobs`，索引记录引用。全文索引初始化失败时，代码保留元数据能力，但不会假定正文全文搜索可用。
- **文件系统** 还承载流量 CA、blob、证据、会话/工具输出和 Skill 文件；各自有不同生命周期，不能在架构图中统一画成“所有数据仅在 PostgreSQL”。
- **外部依赖** 至少分为模型服务、MCP 服务、公开情报来源和通知渠道。宿主工具运行环境与可选 `artex-hostd` 是另外一类本地依赖，不应与远程 API 混为一体。

来源：[manager.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/manager.go)、[traffic.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/traffic/traffic.go)、[mcpdiscover.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/mcpdiscover.go)、[hostenv_api.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/hostenv_api.go)、[notifier.go](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/notifier.go)。

### 架构阅读时容易混淆的地方

1. **Scheduler 与 Notifier 是独立后台循环。** 前者处理计划/配置驱动事件，后者处理通知事件的渠道投递。二者都从 Server 启动，但不是同一个队列。
2. **通知登记是带保存点的尽力写入。** `RecordNotificationEventTx` 失败时允许业务记录继续提交；不能把注释中的“原子一致”扩展为“每条记录一定送达”。投递失败、重试耗尽和没有匹配渠道也有独立状态。
3. **MCP 工具缓存是描述元数据缓存。** 渐进暴露、调用解锁、失败冷却和输出落盘是不同机制，均不能直接记作“语义结果缓存”。
4. **HTTP/SSE MCP wrapper 当前主要处理文本块。** 即使另有浏览器接线，也不能据此宣称所有传输原生保留所有多模态结果。
5. **公开情报的增量能力按来源分别标记。** NVD、GHSA 有可见游标路径；可选资料聚合不能仅凭注释标为已实现完整 watermark 增量筛选。数据质量还需观察实际同步结果。
6. **默认值是代码事实，运行状态是环境事实。** 捕获开关、同步开关、通知开关、后台进程、外部认证等未被检查时应保持“未知/未验证”，不要写成已启用，也不要写成缺失。
7. **静态接线不等于可靠性保证。** 例如代理在 goroutine 内启动，启动错误会记录日志；通知有“部分送达”及“永久失败”；Scheduler 有持久化水位与内存待执行队列。架构文档应保留这些状态边界，而不是笼统写“全流程正常”。

### 本节每日维护的核对点

1. 先记录被核对的仓库提交，再比较这些入口和绑定是否改变：`NewManager`、`Server.New`、`restoreTaskRuntimes`、`wireAgentAugment`、`Server.Handler`。
2. 新能力需找到实现、被谁调用、受到哪些配置条件控制；仅添加文件、配置字段、注释或 UI 项不能升级为“已接线”。
3. 没有运行证据时保持“运行未验证”；如果后续获得环境状态，只登记必要的开关/版本/成功时间与脱敏结果，不把凭据和目标敏感数据写入文档。
4. 删除或改名、存储迁移、触发入口迁移、可选功能门控改变，应同时更新能力表和总架构图中的相关连接。

## 补充用户可见能力

核对基线：`675479c881bf27f6ceebfb3347a2319f0f170dff`。下表区分源码实现、入口接线和部署可用性；本次只读核对，所有运行状态均未验证。

| 模块及高层用途 | 实现与实际接线依据 | 配置／环境依赖 | 运行状态与证据边界 |
| --- | --- | --- | --- |
| 对话与任务内交流：维护独立会话及任务交流记录，支持发送、停止、历史查看和模型选择。 | [Handler 注册对话与任务聊天入口](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[runConversationTurn](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/conversations.go) 解析 ChatAgent，调用 `ca.Chat` 并写入对话活动；[任务聊天处理](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go) 调用 `ma.Chat`，无模型时有简化回退。 | PostgreSQL、有效且启用的 Agent／模型配置、会话文件可写；涉及的外部工具另需各自依赖。 | 已确认后端注册与调用；未进行发送、流式响应、停止或持久化恢复实测。无模型的简化回退不代表完整模型对话可用。 |
| Sidequestion：基于当前上下文快照进行独立旁路问答，保留历史与取消状态。 | [Server.New 初始化](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go) 与 [Handler 注册](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[initSideQuestions／flushSideSnapshots](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/side_questions.go) 接入快照发布及数据库保存；[runSide](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/side_questions.go) 调用服务。[SideQuestionService](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/sidequestion/service.go) 只有模型响应处理，没有工具执行循环。 | PostgreSQL 中的快照／问答状态、可用上下文快照、有效模型提供方及上下文预算。 | 已确认快照和问答服务接线；未验证实际快照覆盖、并发取消或断线恢复。服务按只读上下文问答实现，不能据此推断具备独立任务执行能力。 |
| 任务归档与恢复：将任务相关数据库记录、会话文件及可用的流量／证据材料打包归档，支持恢复和删除归档。 | [Server.New 启动归档 worker](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[Handler 注册](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[startTaskArchiveWorker／runOneTaskArchiveJob](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/task_archives.go) 从数据库领取作业；[archiveTask／restoreTaskArchive](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/task_archives.go) 处理包、校验及恢复。 | PostgreSQL、可写数据目录、足够磁盘空间、兼容的归档格式；流量／证据部分依赖对应存储存在。 | 已确认持久化作业与后台处理接线；未验证真实归档、恢复完整性和失败回滚。任务归档的范围不等于整套部署备份。 |
| 任务模板：保存可复用的任务目标、说明和参数模板，并管理模板记录。 | [Handler 注册模板 CRUD](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[请求校验与列表／创建／修改／删除实现](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/task_templates.go) 调用 PostgreSQL 的模板方法。 | PostgreSQL、符合校验的模板字段，以及适用时有效的关联配置。 | 已确认后端 CRUD 实现与注册；未验证前端套用模板及创建任务的完整交互，不把模板存在直接记为创建流程已经跑通。 |
| 程序自更新与回滚：检查发布版本，暂存新二进制，报告进度并发出重启请求；具备备份回滚入口。 | [Handler 注册](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[updateCheck／updateApply／updateRollback](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/update.go) 接入发布检查、`selfupdate.Stage` 和 `Rollback`；[main 中 Bootstrap](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/cmd/artex/main.go) 及 [重启信号处理](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/cmd/artex/main.go) 构成启动／退出接线。 | 对应平台的发布资产、网络、适用的二进制安装方式、安装位置写权限与外部重启管理；回滚依赖有效备份。 | 已确认源码调用；未执行下载、替换、重启或回滚。进程自身发出退出信号，重新拉起依赖外部运行环境，不能仅凭按钮注册判断更新可用。 |
| 会话接收与查看：管理接收实例，展示连接元数据及有大小上限的内容预览。 | [Server.New 创建并注入 Manager](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[Handler 注册管理入口](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[管理处理函数](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/listeners_api.go) 调用 `listener.Manager`；[会话与快照结构](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/listener/listener.go) 保存预览和状态。 | 本地网络资源、相应运行权限及实例生命周期管理。 | 已确认管理入口和内存状态实现；未启动任何接收实例。现有会话集合为进程内状态，未确认持久化；不据此认定为完整交互式会话管理。 |
| 可选网络中继集成边界：为接收能力提供地址解析，并在特定管理路径中关联外部中继生命周期。 | [Server.New 注入解析器](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go)；[解析及生命周期实现](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/c2_relay.go)；[接收器管理路径](https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/listeners_api.go) 显式关联开始／停止。仅记录既有组件连接关系。 | 适用的可选配置、外部进程和网络环境，以及对应认证条件。 | 已确认静态接线；未验证连接与生命周期。不能推断所有调用接收 Manager 的路径都会自动管理中继，也不能在未检查配置时标记为可用。 |

## 每日维护规则

1. 读取默认分支 HEAD，与本文基线比较。源码未变时复用已核验实现，并检查匹配提交的 CI 是否重跑或状态改变。
2. 读取变化模块及必要调用方、配置解析、持久化/API 与前端接线，更新图、能力行、条件和来源链接。README、CHANGELOG、设计文档用于索引，结论以实现和运行证据为准。
3. 实现、接线、开关、CI 和部署状态分别保留。访问失败时说明受阻位置与最后已验证状态，不把未知填成可用。
4. 有实质变化时更新本文及 README 文档索引，通过独立分支与 PR 提交并保留人工编辑。沿用开放的维护 PR；前一 PR 合并后，从最新默认分支建立下一次维护分支。
5. 没有实质变化时在每日维护结果中报告核验提交与 CI，不制造仅日期改变的提交。Git 历史保留实际修改。
6. 新研究的适配以已核验模块及 API/部署条件为基础；已有压缩、去重、记录、工具接入或报告不重复作为“从无到有”的建议。
7. 更新范围为架构与能力文档。每个运行验证结论需记录环境、时间和来源；静态审阅不替代部署测试。


## 2026-11-17 当前 main 增量核验（Asia/Shanghai）

**核验日期：** 2026-11-17（Asia/Shanghai）。**当前源码：** [`c80301da7cfdfd56291550c07940606a50420744`](https://github.com/MythHack/ARTEX/commit/c80301da7cfdfd56291550c07940606a50420744)。**上一轮核验：** `ca3b706669c035bb48398221861de8e7af9b602c`。GitHub compare 显示 main 前进 **26 个提交**；compare 接口返回 300 条文件路径，结果可能截断，不把 300 解释为完整文件总数。本轮对新增和修改的核心模块、关键调用方及服务端注册路径做分组静态核验；没有逐行审计全部变化文件，也没有读取部署数据库、运行配置或生产实例。

### 本轮新增或显著调整的产品能力

| 能力 | 实现与实际接线 | 配置、依赖和边界 | CI / 运行状态 |
| --- | --- | --- | --- |
| **ARTEX 对外 MCP 服务端** | 新增 `server/mcp_server.go`、`mcp_server_access.go`、`mcp_server_tools.go` 与 `db/api_tokens.go`。独立顶层路由将 `/mcp` 指向 `serveMCP`，不经过 Web 根路由的 HTTP Basic Auth；管理 API 注册令牌、设置和审计入口。实现无状态 Streamable HTTP JSON-RPC，仅支持 initialize、ping、tools/list、tools/call；外部工具为显式白名单。 | `mcp_server_enabled` 默认关闭；仅接受 `Authorization: Bearer` 访问令牌，不接受管理员 JWT、Basic Auth、Cookie 或 URL 参数。令牌明文只在创建时返回，数据库保存 SHA-256 哈希；权限分为 `read`、`task:create`、`task:control`，可限制企业、未结束任务数、每小时创建数、有效期及吊销。另有来源 IP/CIDR 白名单、可信代理解析和 Origin 检查。开启后外部调用可创建真实任务，必须只向可信客户端发放最小权限令牌。 | 源码、API 路由、白名单和令牌存储实现已确认；有 MCP 服务端测试文件。当前 HEAD 未取得可核对的 CI run/attempt；未创建令牌、未调用端点，外部接入和运行未验证。 |
| **MCP 调用审计** | `mcp_server.go` 在外部工具调用路径记录令牌、工具、参数摘要、结果状态、来源地址和时间；数据库保留有界审计记录。 | 依赖 PostgreSQL、MCP 服务端开关、有效令牌和访问策略；审计参数/结果有长度上限，不应理解为完整原始会话录制。 | 实现与调用路径已见；未验证部署中的审计保留、权限和实际请求。 |
| **自主推进 Autopilot** | 新增 `agent/autopilot.go` 的 Planner/Worker 条件提示尾巴；`server/autopilot.go` 在 Engine 启动后运行独立看门狗，周期性检查任务进度；`server/autopilot_extend.go` 在满足条件时追加后续目标。相关上下文通过拦截器注入，`agent/chain.go` 的 `dispatchChainIntent` 由 finding 上报路径调用：仅新建的 critical/high finding 才可能按漏洞维度生成继任意图，重复上报不续跳。 | 任务级 Autopilot 开关与全局默认/停滞时间/延长次数共同影响行为；看门狗进度状态有进程内部分，重启后重新建立。目标续杯有波数上限与进度判断。Auto-finish 对 Autopilot 任务采用不同收尾条件。提示词约束不等于强制执行保证，也不绕过 Guard 或人工审批。 | 相关实现、Engine 启动及 finding 上报调用路径已确认；设置值和任务开关是否在用户部署启用未知。未执行长任务验证、未验证重启后的真实行为。 |
| **原生 HTTP 请求与登录会话** | `agent/tools_http.go` 提供 `http_request` 与 `session_login`；Worker 的基础工具装配使用 `ToolSet.WorkerTools()`，并通过本地执行工具权限集合约束能动目标的工具。Go HTTP 客户端可复用任务级 Cookie Jar；Cookie 采用 Netscape 文件格式，可与 curl 的 `-b/-c` 共享。 | 依赖任务工作目录、网络出口代理及必要的代理 CA；属于有网络副作用的本地执行工具，受 `local_exec` 等执行权限条件限制。Cookie Jar 文件存于任务共享目录，不是 PostgreSQL 会话主数据。 | 实现及 Worker 工具装配已确认；未在用户环境执行登录或请求，代理捕获和 Cookie 持久化运行未验证。 |
| **任务级后台作业** | `agent/tools_jobs.go` 提供 `job_start/job_status/job_output/job_stop`；脚本、日志、退出码和 JSON 元数据写入任务共享目录，后续意图可读取作业状态和输出。 | 依赖 Bash、任务共享目录及本地执行权限。作业通过进程 PID、退出码文件推断状态；这不是数据库队列、分布式 Worker 或 exactly-once 任务系统。进程管理和宿主重启后的状态需要区分。 | 工具实现存在并进入 Worker 工具集；未验证跨进程重启、PID 复用或部署环境中的作业恢复。 |
| **任务共享脚本** | `agent/tools_script.go` 提供 `save_script/run_script`，脚本保存到任务级 `shared/scripts`，可由后续意图复用；执行仍交给 Bash，保留既有拦截/审计路径。 | 文件写入受任务目录、名称清理、大小上限和执行权限约束；脚本内容本身不等于安全或正确，运行仍依赖宿主解释器。 | 实现与工具封装已见；未验证跨意图文件权限及不同操作系统环境。 |
| **HTTP 批量探活与前端 JS 审计工具** | `agent/tools_probe.go` 的 `http_probe` 生成批量探测脚本和紧凑结构化结果；`agent/tools_jsaudit.go` 的 `js_audit` 生成脚本，检查入口 HTML、JS bundle、Source Map、端点和源码材料。 | 两者主要生成/执行本地脚本，不应误记为 Go 服务端内置的网络分析引擎。JS 审计脚本有自身 urllib/TLS 处理逻辑，不能假定其证书验证语义与原生 HTTP 工具相同；所有结果仍需人工/Agent 复核。 | 实现和对应测试文件已见；未在目标或用户部署执行，未验证实际流量捕获、误报和完整性。 |
| **PoC 脚手架与工具输出证据** | `agent/poc_scaffold.go` 提供按漏洞类别生成验证脚本骨架；`agent/tool_outputs.go` 及 Worker 相关路径提供工具输出留存/核对能力。 | 骨架生成不等于 PoC 已执行或漏洞已复现；工具输出与模型撰写的摘要必须区分。工具输出存在也不能单独证明外部事实真实性。 | 源码及测试存在；本轮未执行生成脚本或验证输出证据链。 |
| **漏洞证据门槛与等级** | `agent/finding_recorder.go` 实现高严重度证据长度和机械信号检查；`agent/finding_workflow.go` 为不同 Agent 注入证据分级说明，区分观察与推断、影响证明和未验证范围。 | 当前机械检查使用 HTTP/命令/工具输出等启发式信号，L1-L4 级别是平台证据纪律，不是形式化证明。满足字符串/长度门槛不保证证据真实，仍需核对原始工具输出和流量。 | 实现及测试存在；未验证实际模型遵循程度、误报率或报告质量。 |
| **证据附件与文件交付** | `agent/tools_artifacts.go` 将工作目录文件交给 host 注入的 `ArtifactRecorder`；`server/artifacts.go` 校验任务目录边界、解析符号链接、计算 SHA-256、同任务内容去重并复制到任务附件目录；`server/server.go` 注册列表、下载、预览、自动关联和导出 API。 | 单文件上限 20 MiB、每任务最多 300 个附件、总量 500 MiB；元数据保存在 PostgreSQL，正文文件保存在任务数据目录。附件与工作区文件、流量 blob、报告正文是不同对象。 | Agent/Server/DB 路径和 API 注册已确认；预览可能依赖外部渲染工具。未验证文件系统权限、归档完整性及各格式报告的端到端包含范围。 |
| **登录凭据自检** | `server/authcheck.go` 实现任务登录配置的一次性真实请求检查，支持配置的 GET/POST、表单/JSON 字段和可选 TOTP；`POST /api/tasks/{id}/auth-check` 已注册。响应不返回密码、TOTP 或正文。 | 依赖任务登录配置、目标网络可达及成功判定条件；调用会真实发送登录请求，不是纯静态校验。 | 实现、API 路由和测试文件已见；未发送请求，部署配置和实际判定未知。 |
| **多键漏洞去重数据模型** | 新增 `db/finding_keys.go`，生成 root/CVE/endpoint/param/legacy 等强键，以及 asset_class/name 弱候选键；弱键设计为候选提示，不应仅凭名称/资产类别自动合并。 | 主要键生成与事务写入辅助函数已确认；本轮未完整追完所有 finding 写入、迁移、查询和归并入口，故不能把“键算法存在”写成所有报告均已完成多键自动归并。 | 实现存在；端到端使用范围和部署数据迁移状态待进一步核对。 |
| **安全研究 Skill 与参考材料扩展** | 新增多个 Skill 目录及 SRC 参考材料/规则文件，包含源码研究、二进制/移动端、API 安全、浏览器/前端和报告规范等内容。Server 的 Skill 装配根据 PostgreSQL 中 Agent 的 Skill 绑定清单加载可见目录，并通过 Skill 元工具提供内容。 | 文件存在不代表所有 Agent 默认可见或已绑定；具体可用性取决于目录、数据库绑定、Agent 装配和配置。Skill 内容是方法材料，不是经过验证的分析引擎。 | 文件与装配实现已确认；未读取用户部署中的 Agent-Skill 绑定，不推断全部新 Skill 已启用。 |

### 新增外部 MCP 服务端的调用边界图

该端点与“ARTEX Agent 调用外部 MCP”是两个方向相反的集成面。它不应与 `wireAgentAugment` 的外部 MCP 客户端装配混为一谈。

```mermaid
flowchart LR
  C[外部 MCP 客户端] --> M["POST /mcp"]
  M --> F{服务端开关}
  F -->|关闭| N[404]
  F -->|开启| A[来源白名单 / 可信代理 / Origin]
  A --> T[Bearer Token 校验]
  T --> S[Scope 与企业/任务限额]
  S --> W[显式 MCP 工具白名单]
  W --> D[任务/漏洞/探索图读取]
  W --> X[受权限约束的任务创建与控制]
  W --> L[调用审计]
```

### 本轮重要边界与未验证项

1. **外部 MCP 服务端与内部 MCP 客户端分离。** `/mcp` 是 ARTEX 对外提供的无状态 Streamable HTTP JSON-RPC；`wireAgentAugment` 是 ARTEX Agent 装配并调用外部 MCP。二者有不同开关、认证、权限和生命周期。
2. **Autopilot 不等于自动放行审批。** 自动推进提示、看门狗唤醒、目标续杯和漏洞后续意图均不替代 Guard、拦截策略和人工审批；任务开关、全局设置及运行预算仍影响行为。
3. **后台作业不等于持久化队列。** 作业元数据/日志/退出码写入文件，但运行进程本身不由数据库队列托管；不能宣称分布式恢复或恰好一次执行。
4. **工具输出证据门槛不是证明系统。** 长度、关键字、HTTP 状态或命令片段只能作为机械信号；真实结论仍需来源、上下文、原始响应及独立复核。
5. **多键 finding 辅助逻辑与现有归并功能不能混为一谈。** 强键/弱键语义已在 DB 实现中表达，但本轮未完整审计所有写入和合并路径，端到端范围保留待核。
6. **新增 Skill 文件不等于全 Agent 可见。** 可见性仍由数据库绑定、装配逻辑和具体 Agent 决定。
7. **MCP Token 是独立凭据。** `/mcp` 不受 HTTP Basic Auth 保护，必须按 Bearer Token、来源白名单、可信代理及最小 scope 管理；端点默认关闭不等于用户部署配置已核实。
8. **外部 SDK 边界：** `go.mod` 中 Norma SDK 的工具执行、转录和模型内部语义未在本轮做完整源码审计；只记录 ARTEX 自身可见的调用与注入点。

### 当前 HEAD 的 CI 与验证状态

- 对当前 SHA `c80301da7cfdfd56291550c07940606a50420744` 查询 GitHub PR workflow runs，返回空列表；combined commit statuses 也未返回状态。
- workflow-run 查询接口仅覆盖其说明中的 PR 触发工作流和第一页；因此准确记录为 **“本次未取得与当前 SHA 匹配的可核验 run/attempt 或 status”**，不推断没有其他 Actions 事件，也不标记 CI 通过或失败。
- 本轮未重跑 CI、未运行 ARTEX、未创建 MCP Token、未访问部署数据库或外部模型服务。Build/Vet/Test/前端构建的当前 SHA 状态均为未知；实际部署与运行可用性未知。
- 原四张 Mermaid 图保留不变；本节新增一张外部 MCP 服务端边界图。原有存储边界继续有效：PostgreSQL 主数据、SQLite 流量索引及本地正文/证据/转录/附件文件各自独立。

**当前源码固定链接：** [Engine](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/server/engine.go) · [Autopilot 看门狗](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/server/autopilot.go) · [MCP 服务端](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/server/mcp_server.go) · [MCP 工具白名单](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/server/mcp_server_tools.go) · [API Token](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/db/api_tokens.go) · [原生 HTTP](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/agent/tools_http.go) · [后台作业](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/agent/tools_jobs.go) · [证据附件](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/server/artifacts.go) · [证据门槛](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/agent/finding_recorder.go) · [Skill 装配](https://github.com/MythHack/ARTEX/blob/c80301da7cfdfd56291550c07940606a50420744/server/assembly.go)。

## 本次维护记录

- **2026-11-01**：建立 `675479c8` 的架构与能力基线，记录 LLM/上下文、数据/证据/交付、外围服务的图与矩阵。
- 区分 PostgreSQL 主数据、SQLite 流量索引和本地证据/转录文件。
- 标注 Compactor 模型接线与文档的差异、Word 流量证据装配待核对路径、源码开关与部署状态。
- 记录当前主分支后端测试失败及来源，不扩大为未验证的运行故障结论。

[arch-main]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/cmd/artex/main.go
[arch-server]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/server.go
[arch-manager]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/manager.go
[arch-assembly]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/server/assembly.go
[arch-traffic]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/traffic/traffic.go
[arch-evidence]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/evidence/store.go
[arch-ci]: https://github.com/MythHack/ARTEX/blob/675479c881bf27f6ceebfb3347a2319f0f170dff/.github/workflows/ci.yml
[arch-ci-run]: https://github.com/MythHack/ARTEX/actions/runs/41221669712
[arch-ci-backend]: https://github.com/MythHack/ARTEX/actions/runs/41221669712/job/124597328221
[arch-ci-frontend]: https://github.com/MythHack/ARTEX/actions/runs/41221669712/job/124597328044
