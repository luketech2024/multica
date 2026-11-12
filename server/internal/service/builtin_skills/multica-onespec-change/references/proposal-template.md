---
id: <本地 feature-name>
status: draft # 状态流转: draft -> designing -> implementing -> completed
author: <user>
createdAt: <ISO 8601 时间>
updatedAt: <ISO 8601 时间>
---

# 变更提案: [需求标题]

## 1. 需求背景
> 来源: [本地 PRD 路径 / 用户提供的需求正文]
> 优先级: [用户明确提供；未提供则待确认]

[需求摘要与原始来源引用]

## 2. 变更详情

### 2.1 核心变更点
**[功能/模块 A]**
- **现状**： [当前逻辑/无]
- **目标**： [新逻辑，中文 EARS 条目列表，如「当…时，bfs-bemp 应…」]
- **原因**： [需求/工作项支撑]
- **影响**： [风险、联调、兼容性等；无则写「无」]

### 2.2 风险与应对
- 风险点: ...
- 回滚策略: ...

## 3. 规范变更规划
> **Spec 编写准则**:
> - ✅ **需要 Spec**: 涉及 **核心业务逻辑变更**、**数据模型修改**、**API 接口变动** 或 **复杂算法调整**。
> - ❌ **无需 Spec**: 纯 UI 样式调整、文案修改、简单 Bug 修复、日志/监控埋点等非业务逻辑变更。

- [ ] **Added**: `specs/user/auth.md` - 新增登录接口
- [ ] **Modified**: `specs/order/process.md` - 修改订单状态流转

## 4. 验证计划
- [ ] 单元测试覆盖核心逻辑
- [ ] 验证场景 A...

## 5. 下一步行动（Next Step）
1. ✅ 已从本地 PRD 或用户提供的需求正文建立需求上下文
2. ✅ 已在 `specs/` 目录下自动生成 Delta Spec 草稿（基于上方「规范变更规划」列表）
3. 🛑 **ACTION REQUIRED**: 请审查「2. 变更详情」章节，确认 **现状/目标** 与 PRD 一致。
4. 🛑 **ACTION REQUIRED**: 请审查 `specs/*.md` 草稿，按需补充或修正 **中文 EARS** 与验收标准。
5. 🚀 **进入下一阶段**: 审查确认后，根据变更复杂度决策：
    - 简单变更 → `$onespec-task <feature-name> --no-design`
    - 复杂变更 → `$onespec-design <feature-name>`
