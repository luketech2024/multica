"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceSlug } from "@multica/core/paths";
import { useWorkspaceId } from "@multica/core/hooks";
import { agentListOptions } from "@multica/core/workspace/queries";
import { runtimeListOptions } from "@multica/core/runtimes";
import {
  projectRoleOrder,
  initializeDefaultTeam,
  type ProjectRoles,
  type ProjectRoleKey,
} from "@multica/core/workbench/configuration";
import { Switch } from "@multica/ui/components/ui/switch";
import { Button } from "@multica/ui/components/ui/button";
import { ConfigurationSelect } from "./configuration-select";
import { useT } from "../../i18n";

export function ProjectRolesFields({
  config,
  onChange,
  disabled = false,
  onBusyChange,
}: {
  config: ProjectRoles["config"];
  onChange: (config: ProjectRoles["config"]) => void;
  disabled?: boolean;
  onBusyChange?: (busy: boolean) => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const slug = useWorkspaceSlug();
  const qc = useQueryClient();
  const [defaultRuntime, setDefaultRuntime] = useState("");
  const [initializing, setInitializing] = useState(false);
  const [error, setError] = useState<string>();
  const locked = disabled || initializing;
  const agents = useQuery(agentListOptions(wsId));
  const runtimes = useQuery(runtimeListOptions(wsId));
  const activeAgents = (agents.data ?? []).filter((agent) => !agent.archived_at);
  const runtimeItems = (runtimes.data ?? []).map((runtime) => ({
    value: runtime.id,
    label: runtime.name,
  }));
  const onlineRuntimes = (runtimes.data ?? []).filter((runtime) => runtime.status === "online");
  const selectedRuntime = defaultRuntime || (onlineRuntimes.length === 1 ? onlineRuntimes[0]!.id : "");
  const hasMissingAgents = projectRoleOrder.some((role) => config.roles[role].enabled && !config.roles[role].agent_id);
  const initialize = async () => {
    if (!slug || !selectedRuntime || initializing) return;
    setInitializing(true);
    onBusyChange?.(true);
    setError(undefined);
    try {
      await initializeDefaultTeam({ config, runtimeId: selectedRuntime, workspaceId: wsId, workspaceSlug: slug, onProgress: onChange });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t(($) => $.workbench.load_failed));
    } finally {
      await qc.invalidateQueries({ queryKey: agentListOptions(wsId).queryKey });
      setInitializing(false);
      onBusyChange?.(false);
    }
  };
  const bindingItems = (
    items: { value: string; label: string }[],
    value: string | null,
  ) => [
    { value: "", label: t(($) => $.workbench.not_set) },
    ...items,
    ...(value && !items.some((item) => item.value === value)
      ? [{ value, label: `${t(($) => $.roles.unavailable)} (${value})` }]
      : []),
  ];
  const change = (
    role: ProjectRoleKey,
    patch: Partial<ProjectRoles["config"]["roles"][ProjectRoleKey]>,
  ) => {
    onChange({
      ...config,
      roles: { ...config.roles, [role]: { ...config.roles[role], ...patch } },
    });
  };

  return (
    <div className="space-y-2">
      {hasMissingAgents && (
        <div className="space-y-2 rounded-xl border bg-surface p-3">
          <ConfigurationSelect
            label={t(($) => $.roles.default_runtime)}
            value={selectedRuntime}
            items={bindingItems(onlineRuntimes.map((runtime) => ({ value: runtime.id, label: runtime.name })), selectedRuntime)}
            disabled={locked || runtimes.isPending || runtimes.isError}
            onChange={setDefaultRuntime}
          />
          <p className="text-caption text-muted-foreground">{t(($) => $.roles.initialize_hint)}</p>
          <Button variant="outline" disabled={locked || !slug || !onlineRuntimes.some((runtime) => runtime.id === selectedRuntime) || agents.isPending || agents.isError} aria-busy={initializing} onClick={() => void initialize()}>
            {t(($) => initializing ? $.roles.initializing : $.roles.initialize)}
          </Button>
          {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
        </div>
      )}
      {(agents.isError || runtimes.isError) && (
        <p role="alert" className="text-caption text-destructive">
          {t(($) => $.workbench.load_failed)}
        </p>
      )}
      {projectRoleOrder.map((role) => {
        const entry = config.roles[role];
        const selectedAgent = activeAgents.find((agent) => agent.id === entry.agent_id);
        const agentItems = activeAgents
          .filter((agent) => (!entry.runtime_id || agent.runtime_id === entry.runtime_id) && (!agent.system_key?.startsWith("onespec:") || agent.system_key === `onespec:${role}`))
          .map((agent) => ({ value: agent.id, label: agent.name }));
        const compatibleRuntimes = runtimeItems.filter((runtime) =>
          !selectedAgent || runtime.value === selectedAgent.runtime_id,
        );
        return (
          <div
            key={role}
            className="space-y-2 rounded-xl border bg-surface p-3"
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-label">{t(($) => $.roles[role])}</span>
              <Switch
                checked={entry.enabled}
                disabled={locked || role === "sched" || role === "product"}
                aria-label={t(($) => $.roles[role])}
                onCheckedChange={(enabled) => change(role, { enabled })}
              />
            </div>
            {entry.enabled && (
              <div className="grid grid-cols-1 gap-2">
                <ConfigurationSelect
                  label={t(($) => $.roles.agent)}
                  value={entry.agent_id ?? ""}
                  items={bindingItems(agentItems, entry.agent_id)}
                  disabled={locked || agents.isPending || agents.isError}
                  onChange={(value) =>
                    change(role, { agent_id: value || null })
                  }
                />
                <ConfigurationSelect
                  label={t(($) => $.workbench.runtime)}
                  value={entry.runtime_id ?? ""}
                  items={bindingItems(compatibleRuntimes, entry.runtime_id)}
                  disabled={locked || runtimes.isPending || runtimes.isError}
                  onChange={(value) =>
                    change(role, { runtime_id: value || null })
                  }
                />
                <p className="text-caption text-muted-foreground">{t(($) => $.roles.runtime_constraint)}</p>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
