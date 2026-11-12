---
name: ai-readme-workspace
description: Generate and update workspace-level topology, contracts, impact, and integration documentation from multiple child repositories. Use when analyzing relationships or coordinated changes across repositories in one workspace.
---

# 多仓库项目群 AI README 生成器

## 目标与边界

在包含多个子仓库的父目录执行本 Skill，生成项目群知识库。它读取子仓库已有的 `ai-readme` 文档，但默认不修改任何子仓库。

输出目录固定为 `docs/ai-readme-workspace/`，与子仓库的 `docs/ai-readme/` 完全隔离：

```text
workspace/
├── docs/ai-readme-workspace/
│   ├── RULE.mdc
│   ├── 项目群总览.mdc
│   ├── 系统拓扑.mdc
│   ├── 接口契约.mdc
│   ├── 变更影响.mdc
│   └── 联调指南.mdc
├── repo-a/docs/ai-readme/RULE.mdc
└── repo-b/docs/ai-readme/RULE.mdc
```

不要写入 `docs/ai-readme/`、`docs/rules/ai-readme/` 或临时的 `.ai-readme/rules/`。这些路径属于旧版或单仓库 `ai-readme` 的范围。

## 激活时机

- 用户要求理解多个仓库的依赖、接口、变更影响或联调方式。
- 用户要求生成或更新工作区/项目群知识库。
- 用户明确调用 `$ai-readme-workspace`。

## 输入

对每个子仓库，优先读取：

- `docs/ai-readme/RULE.mdc`
- `docs/ai-readme/generated/技术架构.mdc`
- `docs/ai-readme/generated/API接口.mdc`
- `docs/ai-readme/generated/核心流程.mdc`
- `docs/ai-readme/generated/数据层.mdc`
- `docs/ai-readme/generated/开发指南.mdc`

缺少某个文档时标记为待补充，不能把推测写成事实。缺少完整 `docs/ai-readme/` 时，报告该子仓库建议先运行 `$ai-readme`。

## 执行流程

1. 扫描父目录的直接子目录，识别子仓库，并排除 `docs/ai-readme-workspace/`、隐藏目录和构建产物。
2. 读取子仓库知识库，汇总仓库职责、技术栈、接口、调用关系和启动方式。
3. 列出 `RULE.mdc` 与五个项目群文档的创建或更新计划。
4. 写入 `docs/ai-readme-workspace/`，每完成一个文档立即更新该目录 `RULE.mdc` 的导航状态。
5. 校验链接、相对路径、子仓库来源和生成信息，然后汇报缺失信息。
6. 按 [agents-md-section-template.md](assets/agents-md-section-template.md) 创建或更新**工作区根** `AGENTS.md` 受控区块，作为自动读取入口。不存在则创建仅含该区块的文件；已存在则只替换受控区块，保留其余内容。

默认不修改**子仓库**的 `RULE.mdc` 或 `AGENTS.md`。子仓库回写仍需用户单独授权。

## 文档要求

- 使用普通 Markdown；不要写 Cursor 专属的规则 frontmatter。
- 每个生成文档至少包含一个与内容相符的 Mermaid 或 ASCII 图示。
- 对不确定信息使用 `<!-- TODO: 待补充 -->`。
- 所有跨仓库接口、依赖和影响结论注明来源子仓库与文档路径。

## 验收标准

- 所有工作区文档只位于 `docs/ai-readme-workspace/`。
- 工作区根 `AGENTS.md` 含项目群受控区块，指向 `docs/ai-readme-workspace/RULE.mdc`。
- 子仓库仅被读取，除非用户明确授权回写。
- `RULE.mdc` 的导航链接指向同目录的五个项目群文档。
- 每个关系结论可追溯到一个或多个子仓库文档。
- 已存在的项目群文档增量更新，保留用户维护内容并刷新生成信息。

## 文件引用

| 文件 | 用途 |
| --- | --- |
| [references/scanning-subrepos.md](references/scanning-subrepos.md) | 子仓库识别和读取规范 |
| [references/generated-docs-spec.md](references/generated-docs-spec.md) | 输出路径、生成顺序和文档规格 |
| [references/validation-and-update.md](references/validation-and-update.md) | 校验与根 `AGENTS.md` 写入规则 |
| [assets/rule-entry-template.md](assets/rule-entry-template.md) | 工作区 `RULE.mdc` 模板 |
| [assets/doc-templates.md](assets/doc-templates.md) | 五类项目群文档模板 |
| [assets/agents-md-section-template.md](assets/agents-md-section-template.md) | 根 `AGENTS.md` 受控区块 |
