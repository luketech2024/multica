"use client";

import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { AcceptanceCriteria, Issue } from "@multica/core/types";
import { useUpdateIssue } from "@multica/core/issues/mutations";
import { issueStatusCategory } from "@multica/core/issues";
import { projectRoleOrder } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { BusinessTemplateContent } from "../../projects/components/business-template-content";
import { RequirementsChangeControls } from "./requirements-change-controls";
import { ExecutionPlanEditor } from "./execution-plan-editor";

export function RequirementsPanel({ issue }: { issue: Issue }) {
  const { t } = useTranslation("issues");
  const { t: projectT } = useTranslation("projects");
  const update = useUpdateIssue();
  const [draft, setDraft] = useState<{ value: AcceptanceCriteria; revision: number }>();
  const [error, setError] = useState<string>();
  const criteria = draft?.value ?? issue.acceptance_criteria;
  if (!criteria || issue.revision === undefined) return null;

  const category = issueStatusCategory(issue);
  const locked = !!issue.acceptance_criteria?.gate.spec;
  const editingChange = issue.status === "paused" && criteria.change?.phase === "editing";
  const readOnly = locked || (!editingChange && !["todo", "backlog", "unstarted"].includes(category ?? ""));
  const disabled = readOnly || update.isPending;
  const edit = (value: AcceptanceCriteria) => {
    setDraft({ value, revision: draft?.revision ?? issue.revision! });
  };
  const save = async (value: AcceptanceCriteria, confirm = false) => {
    setError(undefined);
    try {
      await update.mutateAsync({
        id: issue.id,
        expected_revision: draft?.revision ?? issue.revision,
        acceptance_criteria: value,
        confirm_requirements: confirm,
      });
      setDraft(undefined);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t(($) => $.requirements.save_failed));
    }
  };

  return (
    <section className="space-y-3 rounded-xl border bg-surface p-3" aria-label={t(($) => $.requirements.title)}>
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-label font-medium">{t(($) => $.requirements.title)}</h3>
        <span className="text-caption text-muted-foreground">{t(($) => $.requirements.version, { version: criteria.version })}</span>
      </div>
      {locked && <p className="text-caption text-success">{t(($) => $.requirements.locked)}</p>}
      {locked && criteria.team && <ExecutionPlanEditor issue={issue} />}
      {(criteria.change || issue.status === "in_progress") && <RequirementsChangeControls issue={issue} />}
      {criteria.template && <details className="text-caption">
        <summary className="cursor-pointer">{criteria.template.content.name} · {t(($) => $.requirements.version, { version: criteria.template.revision })}</summary>
        <div className="mt-2"><BusinessTemplateContent template={criteria.template.content} /></div>
      </details>}
      {criteria.team && <details className="text-caption">
        <summary className="cursor-pointer">{t(($) => $.requirements.team_snapshot, { revision: criteria.team.revision })}</summary>
        <ul className="mt-2 space-y-1">
          {projectRoleOrder.map((role) => <li key={role} className="flex flex-wrap justify-between gap-2">
            <span>{projectT(($) => $.roles[role])}</span>
            <span className="text-muted-foreground">{criteria.team!.config.roles[role].enabled ? t(($) => $.requirements.role_enabled) : t(($) => $.requirements.role_disabled)}</span>
          </li>)}
        </ul>
      </details>}
      {criteria.items.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.requirements.empty)}</p>}
      {criteria.items.map((item, index) => (
        <div key={item.id || `new-${index}`} className="space-y-1">
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              aria-label={t(($) => $.requirements.check, { id: item.id })}
              checked={!!item.checked}
              disabled={disabled || !item.id || !!draft}
              onChange={(event) => {
                const value = { ...criteria, items: criteria.items.map((entry, i) => i === index
                  ? { ...entry, checked: event.target.checked ? { at: "", by: "" } : null }
                  : entry) };
                void save(value);
              }}
            />
            <span className="text-caption text-muted-foreground">{item.id || t(($) => $.requirements.new_item)}</span>
            {!readOnly && <Button variant="ghost" size="xs" disabled={disabled} onClick={() => edit({ ...criteria, items: criteria.items.filter((_, i) => i !== index) })}>{t(($) => $.requirements.remove)}</Button>}
          </div>
          <Textarea
            rows={2}
            className="min-h-0 resize-none"
            aria-label={t(($) => $.requirements.item_text, { id: item.id || index + 1 })}
            value={item.text}
            maxLength={120}
            readOnly={disabled}
            onChange={(event) => edit({ ...criteria, items: criteria.items.map((entry, i) => i === index ? { ...entry, text: event.target.value, checked: null } : entry) })}
          />
        </div>
      ))}
      {!readOnly && <Button variant="outline" size="sm" disabled={disabled || criteria.items.length >= 200} onClick={() => edit({ ...criteria, items: [...criteria.items, { id: "", text: "", origin: "user", covers: [], checked: null, note: null }] })}>{t(($) => $.requirements.add)}</Button>}
      <label className="block space-y-1 text-caption">
        <span>{t(($) => $.requirements.bounds)}</span>
        <Textarea
          value={criteria.bounds.join("\n")}
          readOnly={disabled}
          onChange={(event) => edit({ ...criteria, bounds: event.target.value.split("\n") })}
        />
      </label>
      {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
      {!readOnly && <div className="flex flex-wrap gap-2">
        {draft && <>
          <Button size="sm" variant="outline" disabled={disabled || criteria.items.some((item) => !item.text.trim())} onClick={() => void save(criteria)}>{t(($) => $.requirements.save)}</Button>
          <Button size="sm" variant="ghost" disabled={disabled} onClick={() => { setDraft(undefined); setError(undefined); }}>{t(($) => $.requirements.discard)}</Button>
        </>}
        <Button size="sm" disabled={disabled || !!draft || criteria.items.length === 0 || criteria.items.some((item) => !item.checked)} aria-busy={update.isPending} onClick={() => void save(criteria, true)}>{t(($) => $.requirements.confirm)}</Button>
      </div>}
    </section>
  );
}
