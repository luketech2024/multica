# design.md 与 EARS 的分工

`$onespec-design` 生成的 **`design.md` 不要求、也不应全文使用 EARS**。

## 各节写什么

| 章节 | 写法 | 示例 |
|------|------|------|
| **§0 设计评审摘要** | 口语、短句、列表（中文） | 「后管写库 → 调中台 reload → Redis 对齐 RRateLimiter」 |
| **§1 及之后** | 技术 **HOW**：架构、接口签名、表结构、时序、类/模块名 | `CmbxOutboundRateLimitGate`、`POST /admin/rateLimit/reload` |

## 允许出现的「英文」

- **代码标识符**：类名、方法名、配置键、URL、枚举常量（如 `UNLIMITED`、`fail-open`）
- **不是** EARS 需求句：design 中 **禁止** 大段 `When … shall` 或「当…时应…」重复抄 PRD/Delta Spec

## 需求从哪里来

- **可测行为（EARS）**：`onespec/prd/.../prd.md`、`specs/*.md`
- **实现方案**：`design.md`
- **任务拆解**：`$onespec-task` 主要引用 design §2、§3 等（完整模式）

## 下游

不写 EARS **不会** 影响 `$onespec-task`、`$onespec-implement`、`$onespec-testcase`（测试用例仍从 PRD + Delta Spec 追溯 EARS/验收标准）。
