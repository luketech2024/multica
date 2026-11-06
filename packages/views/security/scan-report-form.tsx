"use client";

import { useState } from "react";
import { useWorkspaceId } from "@multica/core/hooks";
import type { Issue } from "@multica/core/types";
import { useRecordSecurityScan, type SecurityFindingInput, type SecurityScanInput } from "@multica/core/workbench/security";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { ConfigurationSelect } from "../projects/components/configuration-select";
import { useT } from "../i18n";

const categories = ["secrets", "deps", "authz", "injection"] as const;
const severities = ["high", "medium", "low"] as const;
const emptyFinding: SecurityFindingInput = { fingerprint: "", severity: "high", category: "secrets", title: "", cwe: "", location: "", recommendation: "" };

export function ScanReportForm({ task }: { task: Issue }) {
  const { t } = useT("projects");
  const save = useRecordSecurityScan(useWorkspaceId());
  const [fields, setFields] = useState({ run_id: "", artifact_version: "", commit: "", scanned_at: "", report_url: "" });
  const [status, setStatus] = useState<"completed" | "failed">("completed");
  const [covered, setCovered] = useState<string[]>([]);
  const [findings, setFindings] = useState<SecurityFindingInput[]>([]);
  const [savedId, setSavedId] = useState("");
  const [error, setError] = useState("");
  const updateFinding = (index: number, patch: Partial<SecurityFindingInput>) => setFindings((items) => items.map((item, position) => position === index ? { ...item, ...patch } : item));
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!task.revision) return;
    setError("");
    setSavedId("");
    const input: SecurityScanInput = {
      ...fields, scanned_at: new Date(fields.scanned_at).toISOString(), status, categories: covered, findings,
      issue_id: task.id, issue_revision: task.revision, repo_id: task.workbench_handoff?.repo_id ?? "",
    };
    try { const result = await save.mutateAsync(input); setSavedId(result.id); }
    catch (error) { setError(error instanceof Error ? error.message : String(error)); }
  };
  if (task.workbench_handoff?.role !== "sec" || !task.workbench_handoff.repo_id) return null;
  return <details className="rounded-xl border bg-surface p-4">
    <summary className="cursor-pointer font-medium">{t(($) => $.security.record)}</summary>
    <form className="mt-4 space-y-4" onSubmit={submit}>
      <fieldset disabled={save.isPending} className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <label>{t(($) => $.security.run_id)}<Input required maxLength={200} value={fields.run_id} onChange={(event) => setFields({ ...fields, run_id: event.target.value })} /></label>
          <label>{t(($) => $.security.artifact)}<Input required maxLength={200} value={fields.artifact_version} onChange={(event) => setFields({ ...fields, artifact_version: event.target.value })} /></label>
          <label className="sm:col-span-2">{t(($) => $.security.commit)}<Input required pattern="[a-fA-F0-9]{40}|[a-fA-F0-9]{64}" value={fields.commit} onChange={(event) => setFields({ ...fields, commit: event.target.value })} /></label>
          <label>{t(($) => $.security.scanned_at)}<Input required type="datetime-local" step="1" value={fields.scanned_at} onChange={(event) => setFields({ ...fields, scanned_at: event.target.value })} /></label>
          <ConfigurationSelect label={t(($) => $.security.status)} value={status} items={[{ value: "completed", label: t(($) => $.security.completed) }, { value: "failed", label: t(($) => $.security.scan_failed) }]}
            onChange={(value) => { if (value === "completed" || value === "failed") setStatus(value); }} />
          <label className="sm:col-span-2">{t(($) => $.security.report_url)}<Input required type="url" value={fields.report_url} onChange={(event) => setFields({ ...fields, report_url: event.target.value })} /></label>
        </div>
        <fieldset className="space-y-2">
          <legend className="text-caption text-muted-foreground">{t(($) => $.security.coverage)}</legend>
          <div className="flex flex-wrap gap-3">{categories.map((category) => <label key={category} className="flex items-center gap-2"><input type="checkbox" checked={covered.includes(category)} onChange={(event) => setCovered((values) => event.target.checked ? [...values, category] : values.filter((value) => value !== category))} />{t(($) => $.security[category])}</label>)}</div>
        </fieldset>
        {findings.map((finding, index) => <fieldset key={index} className="space-y-3 rounded-xl border p-3">
          <legend className="px-1 text-caption">{index + 1}</legend>
          <div className="grid gap-3 sm:grid-cols-2">
            <label>{t(($) => $.security.fingerprint)}<Input required maxLength={200} value={finding.fingerprint} onChange={(event) => updateFinding(index, { fingerprint: event.target.value })} /></label>
            <label>{t(($) => $.security.finding_title)}<Input required maxLength={300} value={finding.title} onChange={(event) => updateFinding(index, { title: event.target.value })} /></label>
            <ConfigurationSelect label={t(($) => $.security.severity)} value={finding.severity} items={severities.map((value) => ({ value, label: t(($) => $.security[value]) }))}
              onChange={(value) => { const severity = severities.find((item) => item === value); if (severity) updateFinding(index, { severity }); }} />
            <ConfigurationSelect label={t(($) => $.security.category)} value={finding.category} items={categories.map((value) => ({ value, label: t(($) => $.security[value]) }))}
              onChange={(value) => { const category = categories.find((item) => item === value); if (category) updateFinding(index, { category }); }} />
            <label>{t(($) => $.security.location)}<Input required maxLength={1024} value={finding.location} onChange={(event) => updateFinding(index, { location: event.target.value })} /></label>
            <label>CWE<Input pattern="CWE-[0-9]+" value={finding.cwe} onChange={(event) => updateFinding(index, { cwe: event.target.value })} /></label>
          </div>
          <label className="block">{t(($) => $.security.recommendation)}<Textarea required maxLength={4000} value={finding.recommendation} onChange={(event) => updateFinding(index, { recommendation: event.target.value })} /></label>
          <Button type="button" variant="outline" size="sm" onClick={() => setFindings((items) => items.filter((_, position) => position !== index))}>{t(($) => $.security.remove_finding)}</Button>
        </fieldset>)}
        <Button type="button" variant="outline" disabled={findings.length >= 200} onClick={() => setFindings((items) => [...items, { ...emptyFinding }])}>{t(($) => $.security.add_finding)}</Button>
      </fieldset>
      {error && <p role="alert" className="text-destructive">{error}</p>}
      {savedId && <p role="status" className="break-all">{t(($) => $.security.recorded)} · {savedId}</p>}
      <Button type="submit" disabled={save.isPending || !task.revision || covered.length !== categories.length} aria-busy={save.isPending}>{t(($) => $.security.record)}</Button>
    </form>
  </details>;
}
