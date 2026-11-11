"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { childIssuesOptions } from "@multica/core/issues/queries";
import { projectResourcesOptions } from "@multica/core/projects";
import { projectRolesOptions, projectRoleOrder, useReviewDelivery } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";

export function DeliveryRecovery({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const { t: projectT } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const mutation = useReviewDelivery(wsId, issue.id);
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const [selected, setSelected] = useState<Record<string, { role: string; repo: string; covers: string[] }>>({});
  const [error, setError] = useState<string>();
  const children = useQuery({ ...childIssuesOptions(wsId, issue.id), enabled: open });
  const repos = useQuery({ ...projectResourcesOptions(wsId, issue.project_id!), enabled: open });
  const roles = useQuery({ ...projectRolesOptions(wsId, issue.project_id!), enabled: open });
  const criteria = issue.acceptance_criteria;
  const locked = !!criteria?.gate.spec;
  const items = locked ? criteria!.items : text.split("\n").filter((line) => line.trim()).map((line, i) => ({
    id: `A-${String((criteria?.next_item_no ?? 1) + i).padStart(2, "0")}`, text: line.trim(),
  }));
  const completed = (children.data ?? []).filter((child) => child.status === "done");
  const team = criteria?.team?.config ?? roles.data?.config;
  if (!["owner", "admin", "member"].includes(member.role ?? "")) return null;
  const save = async () => {
    setError(undefined);
    try {
      await mutation.mutateAsync({
        action: "recover", expected_revision: issue.revision!,
        recovery: {
          items: items.map((item) => item.text),
          tasks: completed.filter((child) => selected[child.id]).map((child) => ({
            issue_id: child.id, revision: child.revision!, role: selected[child.id]!.role,
            repo_id: selected[child.id]!.repo, covers: selected[child.id]!.covers,
          })),
        },
      });
      setOpen(false);
    } catch (cause) { setError(cause instanceof Error ? cause.message : t(($) => $.delivery.save_failed)); }
  };
  return <Dialog open={open} onOpenChange={(value) => {
    if (mutation.isPending) return;
    if (value) {
      setText(criteria?.items.map((item) => item.text).join("\n") ?? "");
      setSelected({}); setError(undefined);
    }
    setOpen(value);
  }}>
    <p className="text-caption text-muted-foreground">{t(($) => $.delivery_review.recovery_hint)}</p>
    <DialogTrigger render={<Button variant="outline" size="sm" />} >{t(($) => $.delivery_review.recover)}</DialogTrigger>
    <DialogContent className="max-h-[85dvh] overflow-y-auto">
      <DialogHeader><DialogTitle>{t(($) => $.delivery_review.recover)}</DialogTitle><DialogDescription>{t(($) => $.delivery_review.recovery_hint)}</DialogDescription></DialogHeader>
      <fieldset disabled={mutation.isPending} className="space-y-3">
        <label className="block space-y-1 text-caption">
          {t(($) => $.delivery_review.recovery_items)}
          <Textarea value={text} readOnly={locked} onChange={(e) => { setText(e.target.value); setSelected({}); }} />
        </label>
        {(children.isPending || repos.isPending || roles.isPending) && <p role="status">{t(($) => $.delivery_review.loading)}</p>}
        {(children.isError || repos.isError || roles.isError) && <Button variant="outline" onClick={() => { void children.refetch(); void repos.refetch(); void roles.refetch(); }}>{t(($) => $.execution_plan.retry)}</Button>}
        {children.isSuccess && completed.length === 0 && <p>{t(($) => $.delivery_review.no_completed_children)}</p>}
        {completed.map((child) => {
          const entry = selected[child.id];
          const edit = (patch: Partial<NonNullable<typeof entry>>) => setSelected({ ...selected, [child.id]: { ...entry!, ...patch } });
          return <div key={child.id} className="space-y-2 rounded-lg border p-3 text-caption">
            <label className="flex gap-2"><input type="checkbox" checked={!!entry} onChange={(e) => {
              if (e.target.checked) setSelected({ ...selected, [child.id]: { role: "", repo: "", covers: [] } });
              else { const next = { ...selected }; delete next[child.id]; setSelected(next); }
            }} />{child.identifier} · {child.title}</label>
            {entry && <>
              <label className="block">{t(($) => $.execution_plan.role)}<select className="block w-full rounded border bg-surface p-2" value={entry.role} onChange={(e) => edit({ role: e.target.value })}>
                <option value="">—</option>{projectRoleOrder.filter((role) => team?.roles[role].enabled).map((role) => <option key={role} value={role}>{projectT(($) => $.roles[role])}</option>)}
              </select></label>
              <label className="block">{t(($) => $.execution_plan.repo)}<select className="block w-full rounded border bg-surface p-2" value={entry.repo} onChange={(e) => edit({ repo: e.target.value })}>
                <option value="">—</option>{repos.data?.map((repo) => <option key={repo.id} value={repo.id}>{repo.label || repo.id}</option>)}
              </select></label>
              {items.map((item) => <label key={item.id} className="flex gap-2"><input type="checkbox" checked={entry.covers.includes(item.id)} onChange={(e) => edit({ covers: e.target.checked ? [...entry.covers, item.id] : entry.covers.filter((id) => id !== item.id) })} />{item.id} · {item.text}</label>)}
            </>}
          </div>;
        })}
        {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
        <Button disabled={mutation.isPending || !items.length || !Object.keys(selected).length || Object.values(selected).some((entry) => !entry.role || !entry.covers.length || entry.role === "dev" && !entry.repo)} onClick={() => void save()}>{t(($) => $.delivery_review.confirm_recovery)}</Button>
      </fieldset>
    </DialogContent>
  </Dialog>;
}
