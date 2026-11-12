# 设计生成规范

## 1. 设计重点规范

**接口签名原则**：
- 定义 HTTP Method、路径、Request/Response 数据结构（字段名 + 类型）
- 枚举业务异常类型及 HTTP 状态码
- 禁止写方法体、SQL 实现、循环/条件逻辑代码

**数据模型要求**：
- 列出新增/修改字段：字段名、类型、约束、注释
- 说明索引策略及理由（查询频率高的字段必须说明）
- 标注事务边界（哪些操作在同一事务内）

**业务逻辑深度**：
- 描述核心判断规则（幂等键、并发锁、超时策略等）
- 列出异常场景及系统预期行为（非代码实现，是行为描述）

---

## 2. 代码分析规则

### 2.1 发现深度决策

| 条件 | 发现深度 | 读取目标 |
|------|---------|---------|
| 跨 3+ 个服务 / 涉及核心链路 | 全面 | Controller、Service、DAO + 关联服务接口 |
| 单服务内 2-3 个模块变更 | 轻量 | 受影响 Service + 直接依赖 DAO |
| 单方法 / 单字段变更 | 极少 | 仅直接涉及文件 |

### 2.2 搜索范围约束

**搜索方式**（按优先级）：
1. **结构认知**：先确认模块目录结构（如 `ls -R src/modules`），避免盲目搜索
2. **L1 精确查找（优先）**：已知或能推测出类名/文件名时，必须使用 `file search` 或 `rg`
3. **L2 语义搜索（兜底）**：仅在无法推测名称时使用 semantic/code search when available（每次聚焦单一问题）
4. **读取约束**：命中文件后用 targeted file reads of the relevant line range 精读相关片段，禁止全目录扫描或整文件 Read

| 约束项 | 规则 |
|--------|------|
| 文件数量上限 | 搜索超 5 个文件时，取相关度最高的 3 个；其余仅记录路径，不读取 |
| 单文件行数 | 超 300 行时，只读关键词上下文（前后各 20 行） |
| 跳过文件 | `*Test.java`、`*.spec.ts`、`*_test.go` 等测试文件 |
| 历史去重 | 检查对话历史，已读取的文件不得重复 Read |

---

## 3. 类型安全约束

| 语言 | 要求 |
|------|------|
| Java | 使用显式类型/接口，避免原始类型；返回值封装为 `Result<T>` |
| TypeScript | 禁止 `any`；首选精确类型与泛型；接口用 `interface` 声明 |
| Python | 提供类型提示（`def foo(x: int) -> str`）；边界处验证输入 |

---

## 4. 接口设计示例（Few-shot）

**示例 1（REST API 签名）**：

```
POST /api/v1/orders/batch-export
Request:  { "dateRange": { "start": "ISO8601", "end": "ISO8601" }, "format": "csv|excel" }
Response: { "code": 200, "data": { "taskId": "string", "downloadUrl": "string" } }
约束：异步下载，返回 taskId 供轮询，不阻塞主线程
```

**示例 2（数据模型变更）**：

```
新增字段：t_order.export_status  TINYINT(1)  DEFAULT 0  COMMENT '0-未导出 1-导出中 2-已导出'
索引：    INDEX idx_export_status (export_status, create_time)
理由：导出任务按状态+时间查询频率高，需覆盖索引避免全表扫描
```

---

## 5. 输出质量检查项

生成 `design.md` 前确认：

- [ ] 所有新增/修改接口均有完整签名（Method、Path、Request、Response）
- [ ] 数据模型变更包含字段定义与索引说明
- [ ] 核心业务逻辑描述了事务边界和异常场景
- [ ] 无具体实现代码（无方法体、无 SQL 实现、无算法代码）
- [ ] 涉及外部依赖时已用 web search/the in-app browser or web search 确认最新 API
