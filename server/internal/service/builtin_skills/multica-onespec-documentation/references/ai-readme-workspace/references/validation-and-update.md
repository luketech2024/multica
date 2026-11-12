# 校验与根 AGENTS.md 写入

## 生成后校验

- `RULE.mdc` 与五个关系文档均位于 `docs/ai-readme-workspace/`。
- `RULE.mdc` 的六个导航链接存在且可解析。
- 每个关系结论标有来源子仓库和文档路径；未知信息使用 `<!-- TODO: 待补充 -->`。
- 每个生成文档至少包含一个符合 Mermaid 语法的关系图或 ASCII 图。
- 工作区根 `AGENTS.md` 含项目群受控区块。
- 本次执行未修改任何子仓库文件，除非用户已明确要求回写。

汇报结果时列出：成功生成或更新的文档、根 `AGENTS.md` 写入结果、缺少 `ai-readme` 的子仓库、缺失资料和未确认的关系。

## 根 AGENTS.md 项目群链接（默认执行）

文档生成完成后，必须把项目群入口写到工作区根 `AGENTS.md`，供自动读取。

- 使用 `<!-- ai-readme-workspace:start -->` 和 `<!-- ai-readme-workspace:end -->` 标识受控区块。
- 不存在根 `AGENTS.md` 时，创建仅含受控区块的最小文件。
- 已存在受控区块时，只替换两标记之间的内容。
- 没有受控区块时，追加该区块，保留所有原有内容。
- 不重写、删除或移动根 `AGENTS.md` 的其他内容。
- 默认不改子仓库的 `AGENTS.md`、`RULE.mdc` 或 `docs/ai-readme/`。
- 若用户要求修改子仓库，先列出精确目标和相对链接，再执行修改。

受控区块模板见 [assets/agents-md-section-template.md](../assets/agents-md-section-template.md)。
