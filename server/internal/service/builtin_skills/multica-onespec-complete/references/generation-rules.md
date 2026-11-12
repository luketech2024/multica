# OneSpec 归档规则

1. 合并前列出 `onespec/changes/<feature-name>/specs/` 与 `onespec/specs/` 的重名文件。
2. 获得用户「归档」确认后，将 Delta Spec 写入 `onespec/specs/`，再移动整个变更目录。
3. 归档目标是 `onespec/archive/<yyyyMMdd>-<feature-name>/`；当天同名目录已存在时停止并要求用户选择新的 feature-name 或手动处理冲突。
4. 将归档后的 `proposal.md` frontmatter 中 `status` 设置为 `completed`，不改业务正文。
