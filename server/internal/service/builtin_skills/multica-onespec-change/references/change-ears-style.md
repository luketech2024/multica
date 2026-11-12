# 变更提案与 Delta Spec 写法（中文 EARS）

`onespec-change` 生成的 **proposal.md §2「目标」** 与 **`specs/*.md` 需求描述** 须与 PRD（`onespec-generate-prd`）对齐，使用 **中文 EARS**。变更点四段式：**现状 / 目标 / 原因 / 影响**（不用 From/To/Reason/Impact）。基础句式见 [prd-ears-style.md](prd-ears-style.md)；本节补充 **变更/多服务** 场景。

## 句式（按场景选用）

| 类型 | 模板 | 示例 |
|------|------|------|
| 事件驱动 | 当 **[条件]** 时，**[服务/模块名]** 应 **[行为]** | 当出站渠道为 cmbx 且存在有效限额配置时，**bfs-bemp** 应在发起银行请求前执行并发控制 |
| 前置/否定 | 若 **[条件]**，**[服务/模块名]** 应 **[行为]** | 若 Redis 在限流路径不可用，**bfs-bemp** 应放行本次出站并记录告警 |
| 状态驱动 | 在 **[持续状态/前提]** 下，**[服务/模块名]** 应 **[行为]** | 在配置重载窗口内，**bfs-bemp** 应以已发布快照语义为准 |
| 角色动作 | 当 **[条件]** 时，**[运营用户/角色]** 应能 **[行为]** | 当未完成主体创建时，**运营用户** 应无法提交子接口限额 |

## 主语选择

- **优先**使用仓库/服务名：`bfs-bemp`、`bfs-bemp-admin`、`bfs-bemp-web`，或 proposal 中约定的模块称谓。
- **避免**单独使用「系统」「后端」等模糊主语（除非 PRD 已统一如此）。
- 与 PRD 中「系统应…」并存时：Delta Spec 可细化到具体服务，须在范围说明中注明与 PRD 的对应关系。

## 禁止

- 英文 **`*When* … shall`**、`**shall**`、`**must**`（与 PRD §4 一致）
- 在 proposal/spec 中写 **类名、表名、HTTP 路径、框架 API** 等 HOW（留给 `design.md`）

## 与 PRD、design 的分工

| 文档 | 内容 |
|------|------|
| `onespec/prd/.../prd.md` | 产品向 WHAT + 中文 EARS 验收 |
| `proposal.md` + `specs/*.md` | 研发向 WHAT + 中文 EARS（可测、可追溯到模块） |
| `design.md` | §0 口语摘要 + §1+ **HOW**（**不要求** EARS，见 `onespec-design`） |

## 下游说明

`$onespec-task`、`$onespec-implement` 不按英文 `shall` 解析；中文 EARS 与英文 EARS **语义等价**，不影响后续流程。
