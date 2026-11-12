# 任务格式模板（轻量模式）

> 轻量模式仅使用主任务格式，不使用子任务结构。

## 主任务格式

```markdown
- [ ] {{NUMBER}}. {{TASK_DESCRIPTION}}{{PARALLEL_MARK}}
  - 需求：{{REQUIREMENT_IDS}}
  - 预计工时：{{HOURS}}h
  - **变更说明**：
    - `{{FILE_PATH}}`: {{CHANGE_DESC}}
```

> **并行标记**：可并行执行的任务追加 ` (P)`；`--sequential` 模式下省略。
>
> **变更说明**：按实际情况填写；涉及多文件时逐行列出，不涉及的类型省略。
