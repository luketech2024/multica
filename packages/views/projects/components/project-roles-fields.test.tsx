import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { projectRoleOrder, type ProjectRoles } from "@multica/core/workbench/configuration";
import { renderWithI18n } from "../../test/i18n";
import { ProjectRolesFields } from "./project-roles-fields";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/api", () => ({ api: {
  listAgents: async () => [
    { id: "agent-a", name: "Agent A", runtime_id: "runtime-a", archived_at: null },
    { id: "agent-b", name: "Agent B", runtime_id: "runtime-b", archived_at: null },
  ],
  listRuntimes: async () => [{ id: "runtime-a", name: "Runtime A" }, { id: "runtime-b", name: "Runtime B" }],
} }));
vi.mock("./configuration-select", () => ({ ConfigurationSelect: ({ label, value, items, onChange, disabled }: {
  label: string; value: string; items: { value: string; label: string }[]; onChange: (value: string) => void; disabled: boolean;
}) => <select aria-label={label} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
  {items.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
</select> }));
afterEach(cleanup);
function Form({ mismatched }: { mismatched: boolean }) {
  const [config, setConfig] = useState<ProjectRoles["config"]>({ v: 1, roles: Object.fromEntries(
    projectRoleOrder.map((role) => [role, { enabled: true, agent_id: role === "sched" ? "agent-a" : null, runtime_id: role === "sched" ? mismatched ? "runtime-b" : "runtime-a" : null }]),
  ) as ProjectRoles["config"]["roles"] });
  return <ProjectRolesFields config={config} onChange={setConfig} />;
}
function mount(mismatched = false) {
  renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <Form mismatched={mismatched} />
  </QueryClientProvider>);
}
it("offers agents and runtime bindings that can execute together", async () => {
  mount();
  await screen.findAllByRole("option", { name: "Agent A" });
  const agent = screen.getAllByRole("combobox", { name: "Agent" })[0]!;
  const runtime = screen.getAllByRole("combobox", { name: "Runtime" })[0]!;
  expect(within(agent).queryByRole("option", { name: "Agent B" })).not.toBeInTheDocument();
  expect(within(runtime).queryByRole("option", { name: "Runtime B" })).not.toBeInTheDocument();
});
it("retains a stale selection and lets the user clear the runtime constraint", async () => {
  mount(true);
  await screen.findAllByRole("option", { name: "Agent A" });
  const runtime = screen.getAllByRole("combobox", { name: "Runtime" })[0]!;
  expect(runtime).toHaveValue("runtime-b");
  expect(within(runtime).getByRole("option", { selected: true }).textContent).toContain("runtime-b");
  fireEvent.change(runtime, { target: { value: "" } });
  const agent = screen.getAllByRole("combobox", { name: "Agent" })[0]!;
  expect(within(agent).getByRole("option", { selected: true })).toHaveTextContent("Agent A");
});
