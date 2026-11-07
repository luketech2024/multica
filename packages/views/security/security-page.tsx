"use client";

import { useState } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useCurrentMember } from "@multica/core/permissions";
import { projectListOptions, projectResourcesOptions } from "@multica/core/projects";
import { securityChecksOptions, securityCheckOptions, securityScanOptions, securityStatuses, useSecurityDecision, type SecurityCheck } from "@multica/core/workbench/security";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
import { ConfigurationSelect } from "../projects/components/configuration-select";
import { AppLink, useNavigation } from "../navigation";
import { useLocale, useT } from "../i18n";
import { githubShortLabel } from "../common/github-url";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { WorkbenchCredentials } from "../settings/components/workbench-credentials";
import { AuditPanel } from "./audit-panel";

const severityColors = { high: "text-destructive", medium: "text-warning", low: "text-info" };

export function SecurityPage() {
  const { t } = useT("projects");
  const navigation = useNavigation();
  const paths = useWorkspacePaths();
  const { role } = useCurrentMember(useWorkspaceId());
  const canManage = role === "owner" || role === "admin";
  const tab = ["checks", "credentials", "audit"].find((value) => value === navigation.searchParams.get("tab")) ?? "checks";
  const selectTab = (value: string) => {
    const params = new URLSearchParams(navigation.searchParams);
    params.set("tab", value);
    params.delete("check");
    navigation.replace(`${paths.security()}?${params}`);
  };
  return <Tabs className="h-full min-h-0 gap-0" value={tab} onValueChange={(value) => selectTab(String(value))}>
    <header className="space-y-3 border-b p-4">
      <h1 className="text-title font-semibold">{t(($) => $.security.title)}</h1>
        <TabsList aria-label={t(($) => $.security.title)}>
          <TabsTrigger value="checks">{t(($) => $.audit.checks)}</TabsTrigger>
          {canManage && <TabsTrigger value="credentials">{t(($) => $.audit.credentials)}</TabsTrigger>}
          {canManage && <TabsTrigger value="audit">{t(($) => $.audit.title)}</TabsTrigger>}
        </TabsList>
    </header>
    <TabsContent value="checks" className="flex min-h-0 flex-col"><SecurityChecksPage /></TabsContent>
    <TabsContent value="audit" className="flex min-h-0 flex-col">
      {canManage ? <AuditPanel /> : <p role="alert" className="p-4">{t(($) => $.audit.denied)}</p>}
    </TabsContent>
    <TabsContent value="credentials" className="min-h-0 overflow-auto p-4">
      {canManage ? <WorkbenchCredentials /> : <p role="alert">{t(($) => $.audit.denied)}</p>}
    </TabsContent>
  </Tabs>;
}

function SecurityChecksPage() {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const projectId = navigation.searchParams.get("project") ?? "";
  const status = securityStatuses.find((value) => value === navigation.searchParams.get("status")) ?? "";
  const selectedId = navigation.searchParams.get("check") ?? "";
  const projects = useQuery(projectListOptions(wsId));
  const list = useInfiniteQuery(securityChecksOptions(wsId, projectId, status));
  const selected = useQuery(securityCheckOptions(wsId, selectedId));
  const checks = list.data?.pages.flatMap((page) => page.checks) ?? [];
  const counts = list.data?.pages[0]?.counts ?? [];
  const navigate = (key: string, value: string) => {
    const params = new URLSearchParams(navigation.searchParams);
    if (value) params.set(key, value); else params.delete(key);
    navigation.replace(`${paths.security()}?${params}`);
  };

  return <main className="flex h-full min-h-0 flex-col">
    <header className="flex flex-wrap items-center justify-between gap-3 border-b p-4">
      <h2 className="text-title-sm font-semibold">{t(($) => $.audit.checks)}</h2>
      <Button variant="outline" onClick={() => void list.refetch()} disabled={list.isFetching}>{t(($) => $.security.refresh)}</Button>
    </header>
    <div className="min-h-0 flex-1 space-y-4 overflow-auto p-4">
      <div className="grid max-w-2xl gap-3 sm:grid-cols-2">
        <ConfigurationSelect label={t(($) => $.security.project)} value={projectId} onChange={(value) => navigate("project", value)}
          items={[{ value: "", label: t(($) => $.security.all) }, ...(projects.data ?? []).map((project) => ({ value: project.id, label: project.title }))]} />
        <ConfigurationSelect label={t(($) => $.security.status)} value={status} onChange={(value) => navigate("status", value)}
          items={[{ value: "", label: t(($) => $.security.all) }, ...securityStatuses.map((value) => ({ value, label: t(($) => $.security[value]) }))]} />
      </div>
      <div className="flex flex-wrap gap-2" aria-label={t(($) => $.security.status)}>
        {securityStatuses.map((value) => <Button key={value} variant={status === value ? "brand" : "outline"} size="sm" onClick={() => navigate("status", status === value ? "" : value)}>
          {t(($) => $.security[value])} {counts.filter((entry) => entry.effective_status === value).reduce((sum, entry) => sum + entry.total, 0)}
        </Button>)}
      </div>
      {list.isPending && <div role="status" aria-label={t(($) => $.security.loading)}><Skeleton className="h-40 w-full" /></div>}
      {list.isError && <div role="alert" className="space-y-2"><p>{t(($) => $.security.failed)}</p><Button variant="outline" onClick={() => void list.refetch()}>{t(($) => $.security.retry)}</Button></div>}
      {list.isSuccess && checks.length === 0 && <p className="py-12 text-center text-muted-foreground">{t(($) => $.security.empty)}</p>}
      <ul className="space-y-2">
        {checks.map((check) => <li key={check.id} className="rounded-xl border">
          <button className="flex w-full flex-wrap items-start gap-3 rounded-xl p-4 text-left hover:bg-muted focus-visible:outline-ring" onClick={() => navigate("check", check.id)}>
            <span className={severityColors[check.severity]}>{t(($) => $.security[check.severity])}</span>
            <span className="min-w-0 flex-1 break-words"><span className="text-caption text-muted-foreground">{check.identifier} · {t(($) => $.security[check.category])}</span><span className="block font-medium">{check.title}</span><span className="text-caption text-muted-foreground">{check.location} · {check.artifact_version}</span></span>
            <span className="text-caption">{t(($) => $.security[check.effective_status])}</span>
          </button>
        </li>)}
      </ul>
      {list.hasNextPage && <Button variant="outline" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>{t(($) => $.security.more)}</Button>}
    </div>
    <Dialog open={Boolean(selectedId)} onOpenChange={(open) => { if (!open) navigate("check", ""); }}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader><DialogTitle>{selected.data ? `${selected.data.identifier} ${selected.data.title}` : t(($) => $.security.evidence)}</DialogTitle></DialogHeader>
        {selected.isPending && <Skeleton className="h-40 w-full" />}
        {selected.isError && <div role="alert"><p>{t(($) => $.security.failed)}</p><Button variant="outline" onClick={() => void selected.refetch()}>{t(($) => $.security.retry)}</Button></div>}
        {selected.data && <SecurityCheckDetail key={`${selected.data.id}:${selected.data.revision}`} check={selected.data} />}
      </DialogContent>
    </Dialog>
  </main>;
}

function SecurityCheckDetail({ check }: { check: SecurityCheck }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const paths = useWorkspacePaths();
  const wsId = useWorkspaceId();
  const { role } = useCurrentMember(wsId);
  const mutation = useSecurityDecision(wsId);
  const evidence = useQuery(securityScanOptions(wsId, check.fixed_scan_id ?? check.last_scan_id));
  const resources = useQuery(projectResourcesOptions(wsId, check.project_id));
  const repository = resources.data?.find((resource) => resource.id === check.repo_id);
  const reference = repository?.resource_ref;
  const location = reference && ("url" in reference ? reference.url : reference.local_path);
  const repositoryLabel = repository?.label || (typeof location === "string" ? githubShortLabel(location) : check.repo_id);
  const [action, setAction] = useState<"accept" | "request_recheck" | "recheck">("request_recheck");
  const [reason, setReason] = useState("");
  const [days, setDays] = useState("7");
  const [version, setVersion] = useState(check.fix_version || check.artifact_version);
  const [scan, setScan] = useState("");
  const canAccept = check.severity === "low" && (role === "owner" || role === "admin");
  const actions = [{ value: "request_recheck", label: t(($) => $.security.request_recheck) }];
  if (canAccept) actions.push({ value: "accept", label: t(($) => $.security.accept) });
  if (check.effective_status === "pending_recheck") actions.push({ value: "recheck", label: t(($) => $.security.recheck) });
  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const base = { expected_revision: check.revision };
    const input = action === "accept" ? { ...base, action, reason, days: Number(days) }
      : action === "recheck" ? { ...base, action, scan_id: scan.trim() }
      : { ...base, action, artifact_version: version.trim() };
    await mutation.mutateAsync({ id: check.id, input }).catch(() => undefined);
  };
  return <div className="space-y-4 break-words">
    <p>{t(($) => $.security[check.severity])} · {t(($) => $.security[check.category])} · {t(($) => $.security[check.effective_status])}</p>
    <p className="font-mono text-caption">{check.location} {check.cwe}</p>
    <section><h2 className="font-medium">{t(($) => $.security.recommendation)}</h2><p className="whitespace-pre-wrap">{check.recommendation}</p></section>
    <AppLink className="text-primary hover:underline" href={paths.issueDetail(check.issue_id)}>{t(($) => $.security.source)}</AppLink>
    <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-caption">
      <dt>{t(($) => $.security.repository)}</dt><dd className="break-all">{repositoryLabel}</dd>
      <dt>{t(($) => $.security.artifact)}</dt><dd>{check.artifact_version}</dd>
      <dt>{t(($) => $.security.evidence)}</dt><dd className="break-all">
        {evidence.data ? <><a href={evidence.data.report.report_url} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">{evidence.data.run_id}</a><p>{evidence.data.artifact_version} · {new Date(evidence.data.scanned_at).toLocaleString(locale)}</p><p>{evidence.data.source_commit}</p></> : check.fixed_scan_id ?? check.last_scan_id}
        {evidence.isError && <Button variant="outline" size="sm" onClick={() => void evidence.refetch()}>{t(($) => $.security.retry)}</Button>}
      </dd>
      {check.accepted_until && <><dt>{t(($) => $.security.expires)}</dt><dd>{new Date(check.accepted_until).toLocaleString(locale)}</dd></>}
      {check.acceptance_reason && <><dt>{t(($) => $.security.reason)}</dt><dd>{check.acceptance_reason}</dd></>}
    </dl>
    {check.effective_status !== "fixed" && <form onSubmit={save} className="space-y-3 border-t pt-4">
      <ConfigurationSelect label={t(($) => $.security.status)} value={action} items={actions} disabled={mutation.isPending}
        onChange={(value) => { if (value === "accept" || value === "request_recheck" || value === "recheck") setAction(value); }} />
      {action === "accept" ? <>
        <p className="text-caption text-muted-foreground">{t(($) => $.security.acceptance_help)}</p>
        <label className="block space-y-1">{t(($) => $.security.reason)}<Textarea required maxLength={2000} value={reason} onChange={(event) => setReason(event.target.value)} /></label>
        <ConfigurationSelect label={t(($) => $.security.duration)} value={days} onChange={setDays} items={[7, 30, 90].map((value) => ({ value: String(value), label: new Intl.NumberFormat(locale, { style: "unit", unit: "day", unitDisplay: "long" }).format(value) }))} />
      </> : action === "recheck"
        ? <label className="block space-y-1">{t(($) => $.security.scan_id)}<Input required value={scan} onChange={(event) => setScan(event.target.value)} /></label>
        : <label className="block space-y-1">{t(($) => $.security.artifact)}<Input required maxLength={200} value={version} onChange={(event) => setVersion(event.target.value)} /></label>}
      {mutation.isError && <p role="alert" className="text-destructive">{mutation.error.message}</p>}
      <Button type="submit" disabled={mutation.isPending} aria-busy={mutation.isPending}>{t(($) => $.security.save)}</Button>
    </form>}
  </div>;
}
