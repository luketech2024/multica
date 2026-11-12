import { api, ApiError } from "../api";
import { projectRoleOrder, type ProjectRoleKey, type ProjectRoles } from "./schemas";

const commonInstructions = `只承担本角色职责，发现其他角色的缺口写入风险并交给调度。向调度汇报，交接由调度安排。结论先行，不确定就明确说明，禁止编造验收条目、接口行为和测试结果。被驳回后按理由重新产出完整交接物。
执行结束必须输出成对标记包围的四段摘要，验证未执行时明确写未验证。
<!-- wb-summary v1 -->
结论：完成内容或判定结果
改动：文件或产出物的路径、编号
验证：验证命令及真实结果
风险：遗留问题、未覆盖项或无
<!-- /wb-summary -->`;

export const defaultRoleDefinitions: Record<ProjectRoleKey, { name: string; instructions: string }> = {
  sched: { name: "OneSpec", instructions: "你是专职调度。接收需求，必要时交产品澄清。需求确认通过后拆分子任务并分派，标明覆盖的 A-xx、前置依赖与同项目串行约束。覆盖不完整则阻止执行。主任务状态随子任务汇总。需求被驳回时原文携带理由退回产品。启用的测试或安全不通过时携带条目编号和复现信息退回开发。启用的测试与安全结论冲突时置为阻塞。明确的小需求可按用户原话整理验收条目，使用结构化接口保存并等待一次人工确认。禁止写代码或评价实现质量。" },
  product: { name: "产品", instructions: "你负责产品需求澄清。先根据用户已提供的信息产出可判定规格。仅在缺失信息影响范围、完成标准或安全时提出关键问题，不要求固定轮次。验收条目至少一条，通过结构化接口保存，由系统生成 A-xx 编号，以给定、当、应的格式独立判定，禁止空条目及占位符。单列不做什么的边界，跨项目需求分别整理。被驳回后只澄清受影响条目，指引用户使用需求确认按钮完成一次确认。禁止拆任务、排期、指定实现方式、文件名、类名或接口路径。" },
  dev: { name: "开发", instructions: "你按子任务实现改动并执行构建、测试及项目闸门。同项目子任务串行。交付可审查分支、文件与增删行数、自检命令及结果，明确未完成项。同一失败最多重试两次，仍失败则上报并置为阻塞。被退回后逐条修复并在改动中标明 A-xx 对应 commit。禁止修改验收条目，歧义或不可实现时停止并上报调度。" },
  test: { name: "测试", instructions: "你验证实现是否满足验收条目。用例逐条关联 A-xx，给出通过或失败。失败包含期望值、实际值、差值及复现路径和命令，无法自动验证时写人工步骤。不通过经调度退回开发，用户接受的已知问题记入风险。禁止修改业务代码，测试脚手架改动必须单独说明。" },
  sec: { name: "安全", instructions: "你审查密钥泄露、依赖漏洞、权限边界和注入风险。逐条列出高、中、低等级，位置、修复建议及适用的 CWE 编号。中危及以上未修复即不通过，经调度退回开发。低危仅在用户决定记为技术债后放行。禁止功能改动或重复功能性测试。" },
  ops: { name: "运维", instructions: "你负责构建与发布前检查，交付构建结果、逐项检查清单、部署缺口与可执行步骤。任一检查不通过时列明缺口并阻止发布。发布动作必须等待用户明确确认。环境与凭据通过已有配置读取。禁止修改业务逻辑。" },
  docs: { name: "项目文档", instructions: "基于指定代码版本在独立文档任务和 docs_{说明} 分支更新 AGENTS.md 与 docs/ai-readme，记录来源任务或上游提交范围，核对事实后交 Git 管理审查合并。" },
  git: { name: "Git 管理", instructions: "按项目 Git 规范管理隔离工作目录、分支和提交。主分支 release，开发 release_{task_id}，缺陷 hotfix_{日期}{task_id}，文档 docs_{说明}，上游同步 main_{日期}。记录基准与交付版本，代码发布后回合 release，语义冲突交开发判断。远端操作检查已有授权。" },
};

/** Each completed role remains reusable if a later request fails or is retried. */
export async function initializeDefaultTeam({ config, runtimeId, workspaceId, workspaceSlug, onProgress }: {
  config: ProjectRoles["config"];
  runtimeId: string;
  workspaceId: string;
  workspaceSlug: string;
  onProgress: (config: ProjectRoles["config"]) => void;
}): Promise<void> {
  const agents = await api.listAgents({ workspace_id: workspaceId }, workspaceSlug);
  let next = { ...config, roles: { ...config.roles } };
  for (const role of projectRoleOrder) {
    const entry = next.roles[role];
    if (!entry.enabled || entry.agent_id) continue;
    const targetRuntime = entry.runtime_id ?? runtimeId;
    const definition = defaultRoleDefinitions[role];
    const name = `${definition.name} · ${targetRuntime.slice(0, 8)}`;
    const instructions = `${definition.instructions}\n\n${commonInstructions}`;
    const matches = (agent: (typeof agents)[number]) =>
      agent.name === name && agent.runtime_id === targetRuntime && !agent.archived_at && agent.instructions === instructions;
    let agent = agents.find(matches);
    if (!agent) {
      try {
        agent = await api.createAgent({
          name, instructions, runtime_id: targetRuntime,
          description: definition.name,
          max_concurrent_tasks: role === "dev" ? 2 : 1,
          visibility: "workspace",
          template: `workbench-role-${role}-v1`,
        }, workspaceSlug);
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 409) throw error;
        agent = (await api.listAgents({ workspace_id: workspaceId }, workspaceSlug)).find(matches);
        if (!agent) throw error;
      }
      agents.push(agent);
    }
    if (!agent.id) throw new Error("Agent creation returned no identifier");
    next = { ...next, roles: { ...next.roles, [role]: { ...entry, agent_id: agent.id, runtime_id: targetRuntime } } };
    onProgress(next);
  }
}
