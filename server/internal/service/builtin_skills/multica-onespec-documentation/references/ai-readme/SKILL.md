---
name: ai-readme
description: Generate or incrementally update Codex-facing repository guidance by analyzing the codebase. Use when creating AGENTS.md and supporting docs/ai-readme documentation for a repository.
---

# AI README 生成器

## 概述

本 Skill 用于自动化生成帮助 AI 理解项目的规则文档，采用 **generated/ + manual/** 双目录结构。

> ⚠️ **模型要求**：本 Skill 涉及大量代码分析和文档生成，**强烈推荐使用 a capable reasoning model**执行。

## 生成目标

本 Skill 生成的文档是**面向人和 AI 共同使用的项目介绍手册**——新成员用来快速了解项目全貌，AI 编程助手用来获取代码修改和功能开发的上下文。生成时**详尽优先**（宁可冗余不遗漏，AI 需要充分上下文才能给出高质量建议），优先用表格和 Mermaid 图等结构化格式呈现。

## 激活时机

- 用户首次为项目生成 AI README
- 用户请求更新已有的 AI README 文档
- 用户运行 `ai-readme` 命令

## 核心原则：AI 能力边界

| 目录       | 定位     | 原则                              |
| ---------- | -------- | --------------------------------- |
| generated/ | 技术事实 | AI 能从代码"看到"的客观信息       |
| manual/    | 业务知识 | 需要人"告诉"AI 的主观决策和上下文 |

| 能力 | AI 能做（→ generated/）    | AI 做不了（→ manual/）       |
| ---- | -------------------------- | ---------------------------- |
| 术语 | 提取代码中的变量名、类名   | 解释业务含义、中文翻译       |
| 流程 | 分析代码调用链             | 理解业务意图、边界条件       |
| 结构 | 扫描目录树、依赖关系       | 解释为什么这样划分           |
| 规范 | 从现有代码推断风格（现状） | 定义团队应该遵守什么（规范） |
| 接口 | 提取签名、参数类型         | 说明业务语义、使用场景       |

## 输出目录结构

```
项目根目录/
├── AGENTS.md                            # 面向 AI Agent 的项目指南（根目录；阶段2第一步生成）
│
└── docs/ai-readme/
    ├── 备忘.mdc                          # 用户偏好备忘（固定；每次执行前必读；不存在则创建，不覆盖）
    ├── RULE.mdc                          # 项目知识索引（固定）
    │
    ├── generated/                       # AI 生成（技术事实）
    │   ├── 项目结构.mdc                   # 所有项目
    │   ├── 技术架构.mdc                   # 所有项目
    │   ├── 开发指南.mdc                   # 所有项目
    │   ├── 核心流程.mdc                   # 所有项目
    │   ├── 数据层.mdc                     # 服务端
    │   ├── 状态管理.mdc                   # 前端/客户端
    │   ├── API接口.mdc                    # 服务端/SDK
    │   ├── 组件接口.mdc                   # 前端/UI库
    │   └── 测试框架.mdc                   # 所有项目
    │
    └── manual/                          # 人工维护（业务知识）
        ├── 业务知识.mdc
        ├── 边界定义.mdc
        ├── 团队标准.mdc
        └── 历史经验.mdc
```

## 执行流程总览

按以下 8 个阶段顺序执行：

| 阶段 | 内容                                         | 详细参考                                                                                                                      |
| :--: | -------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
|  0   | 读取（或创建）备忘.mdc，加载用户偏好         | [scanning-and-detection.md](references/scanning-and-detection.md) §0                                                        |
|  1   | 扫描代码仓 + 扫描已存在规则 + 检测运行模式   | [scanning-and-detection.md](references/scanning-and-detection.md) §1                                                        |
|  2   | 制定 TODO LIST（列出待生成/更新的文档清单）  | [document-generation.md](references/document-generation.md)                                                                 |
|  3   | 生成 RULE.mdc + generated/ 文档              | [document-generation.md](references/document-generation.md) + [generated-docs-spec.md](references/generated-docs-spec.md) |
|  4   | 生成 AGENTS.md（项目根目录，每次都重新生成） | [document-generation.md](references/document-generation.md)                                                                 |
|  5   | 生成 manual/ 模板（跳过已存在的文件）        | [validation-and-followup.md](references/validation-and-followup.md) §3                                                      |
|  6   | 统计结果，校验质量，链路一致性检查           | [validation-and-followup.md](references/validation-and-followup.md) §4                                                      |
|  7   | 引导用户提供额外信息，生成补充文档           | [validation-and-followup.md](references/validation-and-followup.md) §5                                                      |

## 关键约束摘要

### 边生成边更新（强制）

**每生成一个文档，必须立即更新 `docs/ai-readme/RULE.mdc` 快速导航列表中对应行的进度状态！**

```
✅ 正确：写入 项目结构.mdc → 更新 RULE.mdc 状态列 [x] → 写入 技术架构.mdc → ...
❌ 错误：写入所有文档 → 最后统一更新 RULE.mdc
```

### 断点续写状态符号

| 符号  | 含义                                 |
| ----- | ------------------------------------ |
| `[ ]` | 未完成，需要生成                     |
| `[x]` | 已完成（追加时间戳）                 |
| `[?]` | 待确认（中间状态）                   |
| `[S]` | 跳过（内置规则、人工维护、或不适用） |

### 禁止事项

- 禁止使用 Emoji 表情符号（`⚠️` 除外）
- 禁止覆盖 `manual/` 目录下已存在的文件
- 禁止覆盖 `备忘.mdc`（除非用户明确要求）

### 文档格式基本要求

1. 使用普通 Markdown，不写 Cursor 专属的规则 frontmatter
2. **至少 1 个图示**：Mermaid 或 ASCII
3. **AI生成标记**（generated/ 目录）：`<!-- AI生成，可根据团队规范更新 -->`

## 验收标准

0. 必须在开始任何扫描/生成前读取 `备忘.mdc`；不存在则先创建
1. 阶段1必须先检测运行模式（全量生成 / 增量更新）
2. 阶段1必须扫描 `docs/ai-readme/` 目录，识别已存在的项目知识文档
3. `docs/ai-readme/RULE.mdc` 是详细知识索引，不创建第二个 `AGENTS.md`
4. 阶段2第一步在项目根目录生成 `AGENTS.md`（无论是否已存在都重新生成）
5. 所有 `.mdc` 文档直接在 `docs/ai-readme/` 目录生成
6. 根据项目类型生成/更新对应的 generated/ 文档
7. 仅创建不存在的 manual/ 模板文件，已存在的不覆盖
8. 每个 generated/ 文档包含至少 1 个图示
9. `docs/ai-readme/RULE.mdc` 使用 Markdown 链接维护文件导航和进度状态
10. 根据代码特征动态推荐补充文档
11. 每生成一个文档必须立即更新 RULE.mdc 快速导航列表中的进度状态
12. 增量更新时：更新后各文档在"结构 → 流程 → 接口"链路上描述一致
13. 根 `AGENTS.md` 必须链接到 `docs/ai-readme/RULE.mdc`；只对已安装的标准 Skill 写入必用约束，缺少时在结果中提示安装。

## 文件引用索引

| 文件                                                                             | 内容说明                                               |
| -------------------------------------------------------------------------------- | ------------------------------------------------------ |
| [references/scanning-and-detection.md](references/scanning-and-detection.md)   | 阶段0+1：偏好读取、扫描、模式检测、差异分析            |
| [references/document-generation.md](references/document-generation.md)         | 阶段2上半：写入策略、更新规则、格式要求和 AGENTS.md 规格 |
| [references/generated-docs-spec.md](references/generated-docs-spec.md)         | 阶段2下半：每个 generated/ 文档的章节规格和示例        |
| [references/validation-and-followup.md](references/validation-and-followup.md) | 阶段3+4+5：manual/模板、汇总校验、动态推荐、输出清单   |
| [assets/rule-entry-templates.md](./assets/rule-entry-templates.md)                 | RULE.mdc 项目知识索引模板                              |
| [assets/agents-md-template.md](assets/agents-md-template.md)                   | AGENTS.md 的章节规格和生成模板                         |
| [assets/manual-templates.md](assets/manual-templates.md)                       | 备忘.mdc + 4个 manual/ 文档模板                        |
