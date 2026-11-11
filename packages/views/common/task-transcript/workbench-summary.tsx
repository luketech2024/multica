"use client";

import { useState, type ComponentProps } from "react";
import { parseWorkbenchSummary } from "@multica/core/issues/workbench-summary";
import { Button } from "@multica/ui/components/ui/button";
import { ReadonlyContent } from "../../editor";
import { useT } from "../../i18n";
import { redactSecrets } from "./redact";
import { useCurrentIssueRenderContext } from "../../issues/current-issue-render-context";

export function WorkbenchRunSummary({ content, attachments }: Pick<ComponentProps<typeof ReadonlyContent>, "content" | "attachments">) {
  const { t } = useT("issues");
  const issue = useCurrentIssueRenderContext();
  const [expanded, setExpanded] = useState<boolean>();
  const showFullOutput = expanded ?? issue?.status === "blocked";
  const safeContent = redactSecrets(content);
  const summary = parseWorkbenchSummary(safeContent);
  if (!summary) return <ReadonlyContent content={safeContent} attachments={attachments} />;
  const sections = [
    [t(($) => $.run_summary.conclusion), summary.conclusion],
    [t(($) => $.run_summary.changes), summary.changes.join("\n")],
    [t(($) => $.run_summary.validation), summary.validation.join("\n")],
    [t(($) => $.run_summary.risks), summary.risks.join("\n")],
  ];
  return <section className="space-y-3 rounded-xl border bg-surface p-4" aria-label={t(($) => $.run_summary.title)}>
    <dl className="space-y-3 text-body">
      {sections.map(([label, value]) => <div key={label} className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-3">
        <dt className="font-medium text-muted-foreground">{label}</dt>
        <dd className="min-w-0 whitespace-pre-wrap break-words">{value || t(($) => $.run_summary.missing)}</dd>
      </div>)}
    </dl>
    <Button size="sm" variant="ghost" aria-expanded={showFullOutput} onClick={() => setExpanded(!showFullOutput)}>{showFullOutput ? t(($) => $.run_summary.collapse) : t(($) => $.run_summary.expand)}</Button>
    {showFullOutput && <ReadonlyContent content={safeContent} attachments={attachments} />}
  </section>;
}
