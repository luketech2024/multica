# 工作区文档生成规格

## 固定输出位置

所有工作区级文件只能写入：

```text
docs/ai-readme-workspace/
├── RULE.mdc
├── 项目群总览.mdc
├── 系统拓扑.mdc
├── 接口契约.mdc
├── 变更影响.mdc
└── 联调指南.mdc
```

不要写入 `docs/ai-readme/`、`docs/rules/ai-readme/` 或 `.ai-readme/rules/`。若目标路径不可写，停止并报告受限路径。

## 生成顺序

1. 根据扫描结果列出六个文件的状态：`新建`、`需更新`、`已是最新`。
2. 先创建或更新 `RULE.mdc` 的目录结构和导航表。
3. 依次生成五个关系文档；每完成一个，立刻更新 `RULE.mdc` 对应状态和时间。
4. 最后更新 `RULE.mdc` 的生成信息、来源仓库快照和缺失项。

使用普通 Markdown。生成文档应包含 `<!-- AI生成，可根据团队规范更新 -->`、至少一个图示和可追溯的来源说明。

## RULE.mdc

输出：`docs/ai-readme-workspace/RULE.mdc`

内容：

- 项目群定位
- 生成时间和各子仓库分支/提交快照
- 子仓库职责与关系图
- 五个项目群文档的快速导航和状态
- 缺失资料清单

模板见 [assets/rule-entry-template.md](../assets/rule-entry-template.md)。

## 五个关系文档

| 文件 | 核心内容 | 主要来源 |
| --- | --- | --- |
| `项目群总览.mdc` | 仓库职责、技术栈、依赖与架构图 | 各仓库 `RULE.mdc`、技术架构 |
| `系统拓扑.mdc` | 调用关系、数据流、共享基础设施 | API、核心流程、数据层 |
| `接口契约.mdc` | 共享类型、跨仓库接口、版本约定 | API 接口文档 |
| `变更影响.mdc` | 依赖方向、接口变更影响、检查清单 | 依赖与接口引用 |
| `联调指南.mdc` | 启动顺序、命令、端口、联调场景 | 开发指南 |

所有文件均位于 `docs/ai-readme-workspace/`，导航链接使用同目录相对路径。

## 根 AGENTS.md 链接（默认执行）

五个关系文档写完后，必须更新工作区根 `AGENTS.md`：

1. 不存在根 `AGENTS.md` 时，创建仅含受控区块的最小文件。
2. 已存在受控区块时，只替换 `<!-- ai-readme-workspace:start -->` 与 `<!-- ai-readme-workspace:end -->` 之间的内容。
3. 没有受控区块时，追加该区块，保留所有原有内容。

使用 [assets/agents-md-section-template.md](../assets/agents-md-section-template.md)。默认不把链接回写到子仓库文档；此类跨仓库修改需要用户单独明确授权。
