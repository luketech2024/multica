# 实施计划

## 任务格式模板

根据工作拆分选择合适的格式：

### 仅主任务
- [ ] {{NUMBER}}. {{TASK_DESCRIPTION}}{{PARALLEL_MARK}}
  - 需求：{{REQUIREMENT_IDS}}
  - **配置变更**：
    - `{{CONFIG_FILE_PATH}}`: {{CONFIG_CHANGE_DESC}}
  - **后端代码变更**：
    - `{{BACKEND_FILE_PATH}}`: {{BACKEND_CHANGE_DESC}}
  - **前端代码变更**：
    - `{{FRONTEND_FILE_PATH}}`: {{FRONTEND_CHANGE_DESC}}
  - **SQL执行**：
    - `{{SQL_FILE_PATH}}`: {{SQL_CHANGE_DESC}}
  - {{OTHER_DETAIL_ITEM}} *(仅在需要时补充其他细节)*

### 主任务 + 子任务结构
- [ ] {{MAJOR_NUMBER}}. {{MAJOR_TASK_SUMMARY}}
- [ ] {{MAJOR_NUMBER}}.{{SUB_NUMBER}} {{SUB_TASK_DESCRIPTION}}{{SUB_PARALLEL_MARK}}
  - 需求：{{REQUIREMENT_IDS}}
  - **配置变更**：
    - `{{CONFIG_FILE_PATH}}`: {{CONFIG_CHANGE_DESC}}
  - **后端代码变更**：
    - `{{BACKEND_FILE_PATH}}`: {{BACKEND_CHANGE_DESC}}
  - **前端代码变更**：
    - `{{FRONTEND_FILE_PATH}}`: {{FRONTEND_CHANGE_DESC}}
  - **SQL执行**：
    - `{{SQL_FILE_PATH}}`: {{SQL_CHANGE_DESC}}
  - {{OTHER_DETAIL_ITEM}}

> **任务详情说明**：请根据实际情况填写配置、后端代码、前端代码、SQL变更部分。如某类变更不涉及，可省略该分类。
>
> **任务标题格式**：任务标题（`{{TASK_DESCRIPTION}}` 或 `{{SUB_TASK_DESCRIPTION}}`）本身不包含模块名前缀。模块名通过任务所属的 `##` 二级标题自动识别；外部同步时由 `$onespec-sync-tasks` 根据目标平台格式化标题。
>
> **并行标记**：仅对可并行执行的任务追加 ` (P)`。在 `--sequential` 模式下省略该标记。
>
> **可选测试覆盖**：当某个子任务是可延期的测试工作且与验收标准相关时，将复选框标记为 `- [ ]*`，并在细节子弹项中解释所引用的需求。
