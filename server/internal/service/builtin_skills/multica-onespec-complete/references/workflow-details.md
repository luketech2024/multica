---
name: onespec-complete
argument-hint: <feature-name>
---

# OneSpec Complete Workflow

```text
源目录：onespec/changes/<feature-name>/
主规范：onespec/specs/
归档目录：onespec/archive/<yyyyMMdd>-<feature-name>/
```

归档前必须列出任务状态和将覆盖的主规范，等待用户回复「归档」。归档只影响本地文件；外部平台状态需由 `$onespec-sync-status` 单独处理。
