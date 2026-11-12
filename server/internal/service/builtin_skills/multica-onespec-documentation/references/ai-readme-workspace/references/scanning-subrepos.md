# 扫描父目录与子仓库

## 扫描范围

将当前目录作为工作区父目录，只识别其直接子目录中的代码仓库。通过 `.git/`、`package.json`、`pom.xml`、`build.gradle`、`go.mod`、`composer.json`、`requirements.txt` 或 `pyproject.toml` 判断。

排除以下目录：

- 隐藏目录
- `node_modules`、`dist`、`build` 等构建产物
- `docs/ai-readme-workspace/`
- 不含仓库或项目特征文件的目录

## 子仓库知识来源

对子仓库 `<repo>/`，按以下优先级读取：

| 信息 | 路径 |
| --- | --- |
| 项目定位和职责 | `docs/ai-readme/RULE.mdc` |
| 技术栈 | `docs/ai-readme/generated/技术架构.mdc` |
| 对外接口 | `docs/ai-readme/generated/API接口.mdc` |
| 核心链路 | `docs/ai-readme/generated/核心流程.mdc` |
| 数据结构和基础设施 | `docs/ai-readme/generated/数据层.mdc` |
| 启动命令和端口 | `docs/ai-readme/generated/开发指南.mdc` |

处理规则：

1. 缺少 `docs/ai-readme/RULE.mdc`：记录警告，建议在该子仓库执行 `$ai-readme`；仍可从代码和配置提取明确事实。
2. 缺少单个 `generated/` 文档：在对应项目群章节写 `<!-- TODO: 待补充 -->`，不要臆测。
3. 只读子仓库文件。除非用户明确授权回写，不创建、移动或修改子仓库的 `AGENTS.md`、`RULE.mdc` 或其他文档。

## 关系提取

从已读取信息中提取并标注来源：

| 关系 | 主要来源 | 输出 |
| --- | --- | --- |
| 调用关系 | `API接口.mdc`、`核心流程.mdc` | 系统拓扑 |
| 数据流 | `核心流程.mdc`、`数据层.mdc` | 系统拓扑 |
| 接口契约 | `API接口.mdc` | 接口契约 |
| 共享依赖 | `技术架构.mdc`、构建配置 | 项目群总览 |
| 联调顺序 | `开发指南.mdc` | 联调指南 |

输出扫描摘要时，列出每个子仓库的路径、职责、技术栈、已读取文件和缺失文件。
