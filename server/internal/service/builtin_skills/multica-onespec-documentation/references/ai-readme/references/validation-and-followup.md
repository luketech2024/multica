# 校验与后续（阶段 3 + 4 + 5 + 输出清单）

## 阶段 3：生成 manual/ 目录模板

> AI 无法从代码推断的关键信息，需要人工补充。
> 所有文件直接输出到 `docs/ai-readme/` 目录

### 需要创建的模板文件（4个）

| 文件         | 输出路径                                      | 内容                                                            |
| ------------ | --------------------------------------------- | --------------------------------------------------------------- |
| 业务知识.mdc | `docs/ai-readme/manual/业务知识.mdc` | 项目背景、领域术语表、核心业务流程、业务规则与约束              |
| 边界定义.mdc | `docs/ai-readme/manual/边界定义.mdc` | 模块职责矩阵、模块协作规则、跨仓库关联与接口约定                |
| 团队标准.mdc | `docs/ai-readme/manual/团队标准.mdc` | 分支策略、Commit 规范、质量门禁、Code Review 约定               |
| 历史经验.mdc | `docs/ai-readme/manual/历史经验.mdc` | 踩坑记录、问题原因、解决方案、预防措施（AI写代码/做方案前必读） |

### 模板格式

每个模板包含：

1. 章节骨架
2. `<!-- TODO: ... -->` 占位符

模板内容详见 [assets/manual-templates.md](../assets/manual-templates.md)。

---

## 阶段 4：汇总与校验

**输出内容**：

1. **统计信息**：
    - 成功：N 个 AI 生成文档
    - 失败：K 个（附原因）
    - 跳过：X 个（不适用当前项目类型）

2. **快速自检**：
    - 所有文档包含至少 1 个图示
    - 快速导航列表进度状态正确
    - `docs/ai-readme/` 目录结构完整

3. **链路一致性检查**（增量更新后必须执行）：

| 更新的文档 | 必须检查的关联文档 | 检查内容                         |
| ---------- | ------------------ | -------------------------------- |
| 项目结构   | 核心流程           | 是否引用了已移动/删除的模块      |
| 技术架构   | 开发指南           | 启动命令/构建链路是否仍然正确    |
| 状态管理   | 核心流程           | 状态流转描述是否需要调整         |
| API接口    | 核心流程、组件接口 | 调用链描述、SDK 使用示例是否匹配 |

如发现不一致，追加更新相关文档。

---

## 阶段 5：引导用户补充信息（动态推荐）

根据阶段1扫描结果，**动态识别**项目中可能需要补充的文档。

### 检测规则

| 检测到的特征          | 推荐的补充文档 | 需要用户提供的信息           |
| --------------------- | -------------- | ---------------------------- |
| Redis/缓存相关代码    | 缓存策略说明   | 缓存 key 命名规则、过期策略  |
| MQ/Kafka/RabbitMQ     | 消息队列说明   | 队列名称、消息格式、消费逻辑 |
| EventEmitter/事件订阅 | 事件系统说明   | 事件清单、触发时机           |
| Auth/Guard/JWT        | 认证授权说明   | 角色权限矩阵、鉴权流程       |
| WebSocket/SSE         | 实时通信说明   | 消息类型、推送逻辑           |
| 定时任务/Cron         | 定时任务说明   | 任务清单、执行周期           |
| 第三方API调用         | 外部依赖说明   | 依赖服务清单、调用方式       |

### 输出格式（动态生成）

```markdown
## 根据代码分析，推荐生成以下补充文档

检测到项目使用了 Redis 缓存和消息队列，建议补充：

1. [ ] 缓存策略说明
    - 检测到：`src/cache/` 目录、Redis 配置
    - 请提供：缓存 key 命名规则、过期策略

2. [ ] 消息队列说明
    - 检测到：`@nestjs/bull` 依赖、`src/queue/` 目录
    - 请提供：队列名称、消息格式

请选择需要生成的文档，并提供相关信息...
```

---

## 按项目类型的输出清单

### 前端项目

```
备忘.mdc
RULE.mdc
generated/项目结构.mdc
generated/技术架构.mdc
generated/开发指南.mdc
generated/核心流程.mdc      ← 页面导航、用户交互、状态变更流程
generated/状态管理.mdc      ← 前端特有
generated/组件接口.mdc      ← 前端特有
generated/测试框架.mdc
manual/业务知识.mdc
manual/边界定义.mdc
manual/团队标准.mdc
manual/历史经验.mdc         ← AI写代码/做方案前必读
```

### 服务端项目

```
备忘.mdc
RULE.mdc
generated/项目结构.mdc
generated/技术架构.mdc
generated/开发指南.mdc
generated/核心流程.mdc      ← API 调用链、业务处理流程
generated/数据层.mdc        ← 服务端特有
generated/API接口.mdc       ← 服务端特有
generated/测试框架.mdc
manual/业务知识.mdc
manual/边界定义.mdc
manual/团队标准.mdc
manual/历史经验.mdc         ← AI写代码/做方案前必读
```

### 全栈项目

```
备忘.mdc
RULE.mdc
generated/项目结构.mdc
generated/技术架构.mdc
generated/开发指南.mdc
generated/核心流程.mdc      ← 前后端核心业务流程
generated/数据层.mdc        ← 服务端
generated/状态管理.mdc      ← 前端
generated/API接口.mdc       ← 服务端
generated/组件接口.mdc      ← 前端
generated/测试框架.mdc
manual/业务知识.mdc
manual/边界定义.mdc
manual/团队标准.mdc
manual/历史经验.mdc         ← AI写代码/做方案前必读
```

### 客户端项目

```
备忘.mdc
RULE.mdc
generated/项目结构.mdc
generated/技术架构.mdc
generated/开发指南.mdc
generated/核心流程.mdc      ← 应用生命周期、页面跳转、数据同步流程
generated/状态管理.mdc      ← 客户端特有
generated/测试框架.mdc
manual/业务知识.mdc
manual/边界定义.mdc
manual/团队标准.mdc
manual/历史经验.mdc         ← AI写代码/做方案前必读
```

### SDK/库项目

```
备忘.mdc
RULE.mdc
generated/项目结构.mdc
generated/技术架构.mdc
generated/开发指南.mdc
generated/核心流程.mdc      ← 核心 API 使用流程、初始化流程、扩展机制
generated/API接口.mdc       ← SDK特有
generated/测试框架.mdc
manual/业务知识.mdc
manual/边界定义.mdc
manual/团队标准.mdc
manual/历史经验.mdc         ← AI写代码/做方案前必读
```

---

## 文档清单汇总

### generated/ 目录文档（10个）

| 文件         | 内容                                        | 生成方式                            | 适用项目    |
| ------------ | ------------------------------------------- | ----------------------------------- | ----------- |
| 项目结构.mdc | 目录树 + 模块划分 + 依赖图                  | 纯模型，目录扫描 + import 分析      | 所有        |
| 技术架构.mdc | 分层架构 + 技术栈清单                       | 纯模型，解析配置文件 + 分析代码分层 | 所有        |
| 开发指南.mdc | 环境搭建 + 启动命令 + 配置说明              | 模型生成，从 scripts + 配置文件提取 | 所有        |
| 核心流程.mdc | 核心业务流程 + 调用链 + 状态流转 + 依赖分析 | 模型深度分析，入口扫描 + 调用链追踪 | 所有        |
| 数据层.mdc   | 数据库 Schema + Entity 关系 + 数据流转      | 模型生成，从 Entity/Model 读取      | 服务端      |
| 状态管理.mdc | Store 结构 + Action/Mutation                | 模型生成，识别状态管理库使用方式    | 前端/客户端 |
| API接口.mdc  | 路由清单 + 请求/响应类型 + 错误码           | 模型生成，从 Controller 提取签名    | 服务端/SDK  |
| 组件接口.mdc | 组件清单 + Props/Events/Slots               | 模型生成，提取组件接口定义          | 前端/UI库   |
| 测试框架.mdc | 测试工具 + Mock 方式 + 目录结构             | 模型生成，从测试代码提取            | 所有        |

### manual/ 目录模板（4个）

| 文件         | 内容                                                                 |
| ------------ | -------------------------------------------------------------------- |
| 业务知识.mdc | 项目背景（是什么/解决什么/服务谁）+ 领域术语表 + 业务流程 + 业务规则 |
| 边界定义.mdc | 模块职责矩阵 + 协作规则 + 跨仓库关联                                 |
| 团队标准.mdc | 分支策略 + Commit 规范 + 质量门禁 + Review 约定                      |
| 历史经验.mdc | 踩坑记录 + 问题原因 + 解决方案 + 预防措施（AI写代码/做方案前必读）   |
