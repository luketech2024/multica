# Workbench changes on the upstream baseline

Baseline 0d63bfc6556ca8a57512c818b897673b28d381c3, branch feat/workbench-development.

- Shared UI tokens and primitives implement the approved workbench theme. Preserve upstream platform tokens and theme persistence when replaying.
- Issue requirements use the existing acceptance_criteria JSONB column and integer revision compare-and-swap. Regenerate sqlc after replaying issue.sql. Requirements changes and their audit rows commit together.
- Shared issue detail renders the requirements editor. API schema rejects unsupported criteria for editing while retaining the rest of the issue response.
- Agent comments and run transcripts recognize paired wb-summary markers, preserving access to the full original output and attachments.

Validation and incomplete scope are tracked in D:\vibe-coding工作台\开发任务与验证.md. No production deployment or real-agent execution has been performed.
- Deployment targets and encrypted credentials are configured through shared project/settings views and workspace-scoped APIs. Replay migrations 615–619 and the concurrent-index cleanup registrations. MULTICA_WORKBENCH_SECRET_KEY is configured by the operator. Missing environment fields remain incomplete, and self-check/deployment execution is still pending.
- External ticket numbers use the independent issue.external_ticket_no column from migration 620. Preserve validation, non-unique duplicate hints, revision checks, atomic audits, API projections, and subtask restrictions. The system issue identifier remains immutable.
- Project teams use project_resource rows of type workbench_roles with a configuration revision. Public repository lists and counts exclude those rows, and generic resource writes cannot change them. Project and team creation share a transaction and an audit record. Project teams do not require project columns. Migration 621 now belongs to the independent paused-status feature.
- Workbench requirements snapshot project teams on creation. Requirement edits preserve that snapshot, and workbench subtasks inherit their parent's snapshot. Older tasks and native paths without workbench requirements still need integration with the full workflow.
- Shared project creation now uses three steps and preserves failed submissions. Project names are compared case-insensitively after trimming, with transaction-level workspace locking for create/rename. Workbench projects require a 2–40 character name and a description. Business templates, repository stack detection, and the complete scheduling/release workflow remain pending.
- Built-in business templates fork into project_resource rows of type workbench_template. Preserve the server catalog, version checks, transactional creation/audits, and the configuration-resource filters in project_resource.sql. No migration is required.
- Shared project details edit all template fields with revision checks. Manual issue creation prefills project context, criteria and separate bounds while retaining user edits. Workbench issues snapshot the server template alongside the team, and children inherit their parent's snapshots. Full custom template assets, upgrades, default role assignment and outcome gates remain pending.

- Requirements change and execution plans use migrations 621–624. Preserve paused status seeding, the seven-value wire category contract, stop acknowledgements, transactional audits, pending resume records and source-run idempotency. Historical migration fixtures keep their historical seven-status vocabulary.
- POST requirements-change reconciles executor stop results before unlocking edits. Reconfirmation returns to backlog with needs_resplit, and queue triggers prevent the old family from executing. Cancellation restores the most recent confirmed baseline and resumes the original run lineage.
- POST execution-plan creates and retires a complete plan atomically using existing issue numbering and repository primitives. Keep properties.workbench_handoff, version/revision checks, idempotency digests, role/repository/coverage validation, cycle detection, dependency and serial-group database guards. Claim contexts restrict planned subtasks to their selected repository.
- The shared plan editor supports dependencies within the submitted plan and existing same-project tasks, with member-approved saving. Configured role agents are assigned, otherwise the submitting member is assigned. Automatic plan generation, the complete state matrix and outcome/release gates remain pending.
- Planned subtasks expose workbench_handoff as an optional top-level response object in issue builders and WebSocket payloads. Keep scalar custom property schemas intact. The shared detail panel resolves repository and dependency data and avoids presenting edited parent text as historical evidence.
- Persisted plans dispatch through the existing scheduler every ten seconds. Replay migration 625 for serial-group queue reservations. Preserve member/agent access checks, transaction locks, no-repeat dispatch after any run, dependency gating and selected local-repository machine checks. Automatic plan generation and full retry/state gates remain pending.
- Replay migration 626 for workbench run retry budgets and safe retry refusal. Planned runs persist max_retry + 1 total attempts, independent of native network retry extensions. Terminal failure blocks the subtask with atomic failure/status audit, including runtime sweepers. Preserve the existing native retry policy for other tasks.

- Replay migration 627 for manual execution pause. Preserve requirement contents, stop acknowledgement checks, revision-based writes, atomic audit, source-run resume idempotency and scheduler suppression while resuming. Restore the prior business state, including unstarted states. The queue guard must retain migration 623 needs_resplit checks.

- Plan submission validates existing dependency closures, rejects unavailable/cross-project/retiring dependencies and cycles, and keeps draft keys separate from existing issue IDs. Persisted handoffs reuse the scheduler and execution guards. The shared editor reports coverage, repository gaps and cycles before saving. Handoff panels resolve dependencies by issue ID, including tasks outside the current parent.

- Task previews reuse the workspace URL custom property named preview_url and existing property write APIs. Keep that integration name and resolve its UUID from the property catalog. Shared details can enable the field, save/clear URLs and draft addresses from configured preview domains. Paths use immutable project IDs and system issue identifiers. Suggested addresses only become links after an explicit save. Hosting/proxy/iframe work remains deferred under T15.

- Repository stack v3 and delivery settings live in each project_resource reference, with revision-checked member edits and atomic activity audit. Generic resource updates preserve owned profile fields and invalidate evidence on source changes. Replay migration 628 and its concurrent-index cleanup to preserve repository uniqueness independently of profile JSON. Catalog commands are configuration-derived, manual overrides retain provenance, and runtime values describe capabilities rather than fixed hosts. Detection/scan execution and wizard integration remain pending.

- Project creation accepts optional repository_profile per embedded resource and persists each validated initial profile plus its audit in the project transaction. Shared wizard fields edit repository-specific drafts and preserve them after failure. Unedited profiles stay unconfigured. Duplicate or invalid resources roll back the entire create.

- Member-created project requirements default to the product role when empty and scheduling role when populated. Preserve explicit manual/unassigned choices through use_project_assignee, validate agent invocation access, and fall back to the creating member when unconfigured. The shared create form stores assignment origin and restricts new requirements to todo/backlog. Role-specific runtime dispatch remains pending.

- The shared issue sidebar shows workbench pipeline progress from the task team snapshot and current plan issue IDs. Exclude disabled roles and retired/version-mismatched tasks, preserve stop/unknown/resume phases, and show missing evidence as unavailable. Preserve loading/error/retry/unplanned states and child navigation. The standalone pipeline output workspace remains pending.

- CLI issue create accepts repeated criterion/boundary flags, external-ticket and explicit unassigned. Project parent creates carry structured empty requirements for clarification, while existing non-project/subtask callers retain their payload unless opted in. Validate requirement limits and initial states before upload/create; server remains the identifier and confirmation authority.

- Role runtime IDs pin execution to the agent's matching native runtime. Validate configuration, default creation and plan approval. Replay migration 629 to enforce the saved task binding on queue/retry/claim transitions while allowing terminal reports. Shared role selectors filter compatible choices and retain stale values for correction.

- Delivery evidence is stored in requirements JSON with server-owned attribution, requirement/child revisions, parent-description hash and handoff role/repository. Reports are attested by members or the assigned agent. Keep explicit manual provenance until repository/scanner integrations provide verified inputs. New plans clear current evidence while transaction audits retain the previous snapshot.
- Shared delivery review checks current enabled-role evidence and project prohibitions/DoD, restricts decisions to the assigned member, and supports reasoned changes and five-second revision-checked undo. Replay migration 630 to invalidate acceptance on reopen and require a fresh acceptance with completion. Evidence, outcome and audit writes share transactions. Automatic aggregation, PR merge and release gates remain pending.
- Workbench tasks and planned children keep immutable UUID URLs. Identifier links resolve before editable content mounts, preserving comment anchors. Native task identifier canonicalization remains supported. This prevents route replacement from discarding open delivery forms.

- The shared standalone issue pipeline is wired into web and desktop issue UUID routes. Preserve stage/task query selection, current-plan filtering, snapshot roles, delivery evidence scoping, PR/log views and retry/empty/loading states. Detailed test/security/deployment output schemas and automatic collection remain pending.

- The persisted-plan scheduler also reconciles parent execution and delivery review. Current child progress starts the parent, complete fresh evidence advances it to review, and conflicting enabled testing/security conclusions block it. Preserve member ownership or transfer to the requirement-confirming member, transactional status/assignment/audit/inbox writes, and repeat/concurrency suppression. Missing permissions, stale evidence, pauses and requirement changes must not advance acceptance.

- Sidebar query-cache subscriptions batch notifications through notifyManager before notifying React external-store consumers. Keep this boundary so queries created during row rendering cannot synchronously update WorkPanel. The regression and delivery E2E cover the render warning.

- Shared inbox categories preserve URL selection and use current server-projected workbench actions, independently of read state. Batch issue lookup, assignment/status/requirements invalidation and mention-before-dedup filtering preserve pending and mention semantics. Workbench rows open immutable issue routes with comment anchors. Delivery decisions read their own workbench notifications in the same transaction and publish inbox updates. Retention and complete notification-source quieting remain pending.

- Inbox insertion derives workbench provenance from the persisted issue for every notification source and reads routine status notices by default. Action-required notices keep unread state, while clients suppress native banners for workbench provenance. The hourly scheduler prunes workbench notices older than 90 days in bounded batches, including older unmarked issue-linked notices.
- The ten-second workbench sweep reconciles member-assigned requirement-confirmation notices under the parent row lock. Deduplicate by requirement version and recipient, read obsolete notices after confirmation/cancellation/reassignment, and preserve unrelated mentions and manual archives. Backlog requirements also project pending actions. Notifications can lag by one sweep, and unresolved requirements receive a new notice after the previous one expires.

- Configuration audit CSV uses existing activity_log records and a fixed repeatable-read 90-day snapshot with timestamp/id pagination. Admin-only authenticated downloads include export metadata and formula-safe UTF-8 CSV. Complete temporary files before sending response bytes and remove them after use. Replay migration 631 and preserve concurrent-index cleanup registration. The credential settings entry currently exports configuration actions; full security/release coverage and inbox download links remain pending.

- Workbench security uses immutable security_scans plus revisioned security_checks (migrations 632–636). Current-plan security subtasks record attributed reports bound to repository, artifact, commit, revision and scan time. Keep member/assigned-executor provenance explicit until automatic scanners are connected. Same-run replay is idempotent, conflicting content is rejected, and older reports cannot replace newer risks.
- High/medium risks require a newer successful scan of the target artifact and repository, with the fingerprint absent and no later scan superseding it. Only managers may accept low risk for 7/30/90 days with a reason. Artifact changes or severity escalation clear acceptance. Hourly reconciliation reminds managers within 72 hours, deduplicates per expiry/recipient, and audits expiry restoration. Read projections treat elapsed acceptance as unresolved immediately.
- Security evidence protects repository source identity and deletion. Project/workspace deletion owns safety-record cleanup and project deletion removes associated reminders. Preserve the workspace deletion manifest and concurrent-index failure cleanup entries. Shared security pages, sidebar count, diagnostics/search/tab routing, report form, inbox links and six-language copy are wired to web/desktop. Security/release gates, full audit and certificate tabs, export-link expiry and automatic scanning remain pending.

- Security center audit pages share web/desktop routing and expose 7/30/90-day configuration/security filters with timestamp/id pagination. Audit JSON details must remain structured objects. Keep manager authorization on list, export and download.
- Replay migrations 637–639 for private audit export snapshots. Generate CSV and its inbox notification transactionally, limit each stored export to 64 MiB, restrict downloads to the original requester with current manager access, enforce 24-hour expiry and hourly cleanup. Preserve no-store, workspace cleanup and concurrent-index recovery. The original settings CSV endpoint retains configuration scope by default. Credential usage, member permissions, certificates and release events still require their actual producers.

- Workspace permission changes now audit role updates, accepted invitations and removals/leaves in the same transaction as the membership change. Lock and reread the target member before recording previous/new roles. Same-role updates and repeated invitation acceptance must not duplicate events. Preserve cache invalidation and event publication after commit.

- Credential expiry reminders run hourly for the upcoming seven days and deduplicate under the credential lock by expiry and manager recipient. Rotation/deletion archives prior reminders transactionally and publishes archive events after commit. Project deletion cleans related reminders. Inbox pending actions for credentials and risk acceptance derive from current resource expiry/state and current manager permission, independently of read state. Preserve credential deep links, masking and six-language copy.

- Runtime deployment checks use revisioned target JSON and heartbeat claims (migration 640). Preserve workspace/runtime/requester authorization, single concurrent claim, bounded reclaim, stale-result rejection and idempotent reports. Configuration/credential changes clear results. Passed checks expire after 24 hours and queued/running checks display timeout after two minutes. The daemon probes writable directories and required tools without executing service start commands. SSH/platform checks, disk capacity and deployment execution remain pending.

- Project deployment targets now use a shared URL-selected deployment tab with the total target count. Keep environment filtering, unbound repository prompts, configuration details and masked credential references. Complete runtime targets trigger a check after successful form save. Incomplete settings remain saveable. The environment filter is local state and resets to staging on reload.

- Runtime target checks now require measured nonzero available disk capacity. Linux/macOS use filesystem statistics and Windows uses GetDiskFreeSpaceEx. Preserve nullable capacity fields for older API payloads, fail closed for unavailable/exhausted capacity and require older successful checks without measurements to run again. Capacity is displayed in MiB. Artifact-size capacity planning remains pending.

- SSH deployment checks require explicit username, port and pinned SHA256 host fingerprint. Open credential envelopes only after current scope/usage/expiry checks. Commit the request and credential-use audit before network execution, release database locks during the bounded probe, and atomically store its outcome with the same check ID so configuration or credential rotation rejects stale results. Keep secrets out of files, responses and logs. The fixed POSIX probe quotes configured paths, locates service executables without starting them and cleans its temporary write probe. Passphrase-protected keys and Windows SSH shells remain unsupported.

- Workbench status bands share the server table-query predicate across rows, groups and facets. Preserve existing filters, URL-selected band/member scope, exact totals and board-default self-assignment. Running/review target the exact built-in statuses because custom started categories do not distinguish review from blocked. Gantt retains its existing filters.
- Replay migrations 641–642 for server-maintained completion timestamps and the independent concurrent workspace/date index. Reopening clears the clock, non-status edits preserve it and subsequent completion sets it anew. Backfill only proven completion audit transitions, leaving undocumented historical completion times null. Day boundaries come from the client local calendar and server filtering uses a half-open interval.

- Workbench board/list surfaces opt into fixed 72px/92px task cards with shared content. Preserve the existing sortable board wrappers, context menus and peek links. Workbench list mode uses a responsive virtual grid (2/3/4 columns at <1280/1280/1600 viewport widths), retains status cursor pagination and ignores legacy list-group collapse preferences. Hide configurable property toggles for these fixed card layouts, while other embedded surfaces retain their configurable cards.

- Workbench keyboard navigation owns ephemeral focus independently of batch selection. Publish row-major board order and card-flow order, register links and virtual-scroller reveal callbacks, and keep a visible selection ring. J/K move, Enter opens, A opens only review tasks, and Escape clears focus or returns task detail to the task list. Existing peek navigation and user-configured global bindings take priority. Help is a configurable global shortcut, and single-key actions defer to text editors, comboboxes, listboxes and portal menus.


## T7 默认项目入口（2026-11-07）

- 登录完成、引导结束、新建工作区、邀请加入、工作区菜单和搜索切换统一使用 workspace.root，默认进入项目列表。网页根地址重定向、桌面新标签、错误恢复、空标签组均同步。
- 桌面跨工作区默认入口恢复已有会话；显式任务地址继续按地址打开。旧任务标签与历史保持原地址，关闭最后一个标签后新建项目列表标签。
- 核心路径104项、网页代理33项、桌面定向149项、邀请/菜单16项通过。桌面扩展837项首次835通过，2项旧默认页测试调整为显式任务初始页后，该文件5项全部通过。全工作区类型9项及各包定向ESLint通过。
- Chromium首页跳转、项目列表刷新、旧任务地址与查询参数保留32.9s通过，pageerror 0。首次浏览器启动早于服务就绪，连接拒绝，待受管环境健康后补跑通过。
- T7其余侧栏分组/置顶、发布入口与最终双端验收仍待完成。


## T10 卡片流返回位置（2026-11-07）

- 卡片流注册共享滚动容器，网页与桌面通过已有 memento 提供恢复位置，虚拟网格初始定位到保存偏移。
- Chromium 56条任务场景43.0s通过，含第二页加载、虚拟末卡选择、详情打开后返回、滚动偏移误差小于3px及原末卡可见，pageerror 0。
- 类型9项与定向ESLint通过。首轮发现虚拟网格初始索引显式undefined会异常，已改为明确index=0、offset=0默认值，并重启服务补跑通过。
- 原生桌面实机恢复仍待整体双端验收。


## T18 工作区模板与选择合并（2026-11-07）

- 增加工作区模板资产表和两份独立并发索引，迁移至645。模板发布带修订号与同事务审计，内置模板只读，另存副本可编辑。新建项目可选工作区模板并保存来源版本；删除工作区清理自有模板，项目副本独立保留。
- 项目模板区加入另存、编辑来源、新版提示。审阅弹窗左右展示当前副本与新版，逐字段选择采用；也可保留全部项目内容并标记已审阅。合并必须同时通过项目修订号和来源版本校验，现有任务快照保持独立。
- 服务端定向回归1.322s、handler全套race106.052s、workbench1.052s、迁移规则1.706s、迁移执行器27.202s通过。补测非成员/agent拒绝、实际工作区删除清理1.179s通过。
- 初次数据库测试在新迁移应用前运行导致缺表，应用643–645后通过。追加权限测试最初使用仓库不存在的viewer角色，已按实际owner/admin/member模型改为非成员与agent验证。未新增角色。
- 核心schema/选择合并回归、前端项目/六语言374项、全工作区类型9项和定向ESLint通过。Chromium完整链路39.4s通过，含发布v2、只采用DoD、保留项目名称、刷新持久化、另存后新建项目可选，pageerror 0；已检查差异弹窗截图 `.preview/workbench-template-merge.png`。
- 新模板默认启用全部角色，项目仍在团队装配步骤明确选择角色。业务模板保存入口设在项目模板区，发布需要工作区成员身份。仓库检测/扫描预检与整体T18验收仍待完成。

## T22 PR 合并证据基础（2026-11-07）

- 迁移646为GitHub和自托管VCS镜像增加目标分支与合并提交。webhook仅在已合并状态保存完整SHA；缺字段的后续元数据保留已有证据，较旧事件不能覆盖。列表响应和任务PR卡展示目标分支、源提交与合并提交。
- 新增SHA校验、GitHub/Forgejo事件重放、GitLab/Forgejo解析与兼容响应测试。handler全套race105.332s、VCS1.017s、迁移规则1.650s通过，前端定向39项、全工作区类型9项及定向ESLint通过。
- 首次sqlc因默认代理下载超时失败，切换已用代理后生成成功，数据库迁移已应用。当前仅验证合并证据基础，真实仓库合并操作、发布资格与发布执行仍在开发。

## T22 逐仓合并校验（2026-11-07）

- 新增任务合并就绪接口，验收卡展示逐仓PR、目标分支与合并提交。按当前计划读取验收证据，匹配仓库来源、任务关联、源提交和目标分支；同一PR的父/子重复关联去重，多个匹配PR保持阻断。已有关联PR合并后，服务端拒绝撤销验收。
- handler全套race107.332s、workbench1.050s通过；补充旧计划证据排除及仓库URL用例后workbench1.084s通过。前端核心10项、组件与六语言270项、全工作区类型9项及定向ESLint通过。
- Chromium验收、合并证据空态、5秒撤销、刷新和过期证据完整流程1.2m通过，pageerror及React渲染错误均为0。首次组件测试暴露重试文案误用仓库重试，已改为独立六语言重试文案后通过。
- 此批验证的是已保存验收与合并证据的一致性。目标分支最新HEAD/祖先关系、发布单占用和真实合并执行继续开发；本地目录及非默认SSH端口尚需明确远端映射。不会把缺失信息计为就绪。

## T22 发布单基础（2026-11-07）

- 迁移647–653已应用。发布单保存验收及逐仓合并快照，服务端分配REL编号，同请求幂等，同仓提交在同环境只允许占用一次；staging/prod分别占用。创建、占用与审计同事务，工作区删除清理三张表，有发布历史的项目禁止删除。
- 网页与桌面接入发布中心、候选筛选、新建与详情，六语言齐全。默认版本采用服务端UTC日期加发布序号，每单最多100任务，手工tag限制1–64字符。当前订单为草稿，四门禁、自动预检、执行与制品跨环境晋级继续开发。
- 核心105项、界面与六语言282项、类型9项及定向ESLint通过。workbench race1.049s、迁移规则1.609s、迁移执行器25.585s通过。handler全套107.349s仅删除测试夹具漏显式ID失败，修正后删除与交付证据定向race1.385s通过。
- Chromium空候选、两环境切换、刷新、1024×720布局14.9s通过，pageerror 0，截图已检查。有候选创建成功由数据库集成与组件测试覆盖，浏览器完整创建链路待补。桌面实际运行与真实部署联调待后续验证。
- 当前制品占用身份采用仓库×合并提交×环境。最终发布提交、目标分支祖先关系、制品实体与标签跨提交唯一性将在发布预检阶段完善。

## T22 发布门禁与确认（2026-11-07）

- 迁移654保存发布单确认。详情重新检查当前验收与合并证据、安全报告及中高风险、变更/迁移确认、每种交付类型的全部目标配置与24小时自检和凭据。部署阻断提供环境设置入口，网页与桌面共用六语言界面。
- 确认带修订号校验与同事务审计，只接受成员操作。安全角色关闭时要求明确承担风险，角色配置版本变化后旧确认失效。当前显示四门检查，执行阶段将按环境规则消费，尚未连接发布执行。
- handler全套race104.739s、workbench1.049s、迁移规则1.650s通过。覆盖旧版本确认拒绝、agent拒绝、确认审计、扫描版本/提交/失败状态、中危阻断、目标交付类型/环境隔离及过期自检。初次新增测试夹具表名与runtime字段有误，修正后全部通过。
- 前端与六语言267项、核心2项通过，类型9项及定向ESLint通过。增加环境入口后组件与类型复验通过。Chromium完整创建、确认、刷新与1024×720布局39.3s通过，pageerror 0，截图已检查。首次浏览器夹具缺描述、任务状态修订号及异步聚合等待，已补齐；新路由首次编译与含仓库标识的文本断言也已修正。
- 浏览器合并记录由测试夹具模拟外部PR事件，任务确认/拆单/交付/验收/发布单创建使用真实接口。当前扫描按本次版本和合并提交严格匹配；多任务同仓多提交仍需后续制品预检确定最终提交及祖先关系。自动构建、发布执行、真实环境联调继续开发。

## T7 统一侧栏与项目看板（2026-11-07）

- 网页与桌面统一为14个入口，按置顶、个人、工作、AI团队、系统分组。最多置顶3项，移动入口并按本设备的工作区保存偏好。宽栏240px，1024–1279px自动收为64px图标栏并提供悬停名称。
- 任务清单与项目看板使用独立视图偏好。项目看板复用最近访问记录，没有项目时先选择项目，失效项目可以重选或重试。自动化移入项目详情，创建与模板预填当前项目，原全局地址仍可访问。
- 组件/六语言310项、项目页后续回归283项、桌面布局8项通过；最终侧栏27项与核心路径/偏好6项复验通过。类型9项、定向ESLint通过，项目详情保留原有effect依赖警告。
- Chromium完整流程41.8s通过，验证置顶与刷新、项目看板、任务清单、项目自动化、1024×720下64px栏与14入口，pageerror为0。宽窄截图均已检查。末次失败由测试错误假定提示元素的role导致，按共享组件实际data-slot修正后通过。
- 桌面实机、全站视觉与整体开发验收继续待做。发布执行等剩余工作仍在开发。

## T22 构建命令配置（2026-11-07）

- 仓库档案新增build命令字段与人工覆盖入口，复用原有修订号、权限和审计。旧档案缺少build时读为空，用户可按仓库填写构建脚本并保存；模板默认保持空值，实际构建方式由仓库配置确定。
- PostgreSQL定向handler race1.222s、workbench全套race1.046s通过，覆盖构建覆盖值与manual来源的保存及并发旧版本拒绝。前端保存失败草稿与build提交1项、核心schema8项、类型9项及定向ESLint通过。
- 本批完成配置契约。发布预检和构建执行尚未接入，空命令不能作为构建成功证据。

## T25 流水线与拆单三态（2026-11-07）

- 流水线空态增加标题、说明与「去拆单」入口，未确认需求的调度页显示原因及任务清单入口。拆单资源加载与任务右栏流水线使用三行骨架；失败显示原因、影响与重试按钮。
- 仓库加载失败时继续禁止保存计划，重试保留已填写的子任务草稿。空草稿提供下一步提示，流水线没有子任务时合并重复空态提示。六语言已同步。
- 组件与六语言274项通过，类型9项、定向ESLint通过。Chromium33.6s通过未拆单入口、未确认任务、资源请求挂起、503错误、重试与草稿保留，pageerror为0，截图已检查。
- 浏览器通过拦截仓库请求模拟网络失败，正常读取和任务确认使用真实接口。此批关闭所列三态缺口，全站三态与最终桌面实机仍待验收。
