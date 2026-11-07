"use client";

import { useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { auditOptions, auditScopes, useCreateAuditExport } from "@multica/core/workbench/audit";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ConfigurationSelect } from "../projects/components/configuration-select";
import { downloadBlob } from "../editor/utils/mermaid-export";
import { AppLink, useNavigation } from "../navigation";
import { useLocale, useT } from "../i18n";

export function AuditPanel() {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { t } = useT("projects");
  const locale = useLocale();
  const navigation = useNavigation();
  const scope = auditScopes.find((value) => value === navigation.searchParams.get("scope")) ?? "all";
  const days = [7, 30, 90].find((value) => String(value) === navigation.searchParams.get("days")) ?? 90;
  const exportId = navigation.searchParams.get("export") ?? "";
  const records = useInfiniteQuery(auditOptions(wsId, scope, days));
  const create = useCreateAuditExport(wsId);
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState("");
  const actionLabels: Record<string, string> = t(($) => $.audit.actions, { returnObjects: true });
  const navigate = (key: string, value: string) => {
    const params = new URLSearchParams(navigation.searchParams);
    params.set(key, value);
    navigation.replace(`${paths.security()}?${params}`);
  };
  const download = async () => {
    setDownloading(true);
    setDownloadError("");
    try {
      downloadBlob(await api.downloadWorkbenchAuditExport(exportId), "workbench-audit.csv");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : String(error));
    } finally {
      setDownloading(false);
    }
  };
  return <section className="min-h-0 flex-1 space-y-4 overflow-auto p-4" aria-label={t(($) => $.audit.title)}>
    <div className="flex flex-wrap items-end gap-3">
      <ConfigurationSelect label={t(($) => $.audit.scope)} value={scope} onChange={(value) => navigate("scope", value)}
        items={auditScopes.map((value) => ({ value, label: t(($) => $.audit[value]) }))} />
      <ConfigurationSelect label={t(($) => $.audit.range)} value={String(days)} onChange={(value) => navigate("days", value)}
        items={[7, 30, 90].map((value) => ({ value: String(value), label: t(($) => $.audit.days, { count: value }) }))} />
      <Button variant="outline" disabled={records.isFetching} onClick={() => void records.refetch()}>{t(($) => $.audit.refresh)}</Button>
      <Button disabled={create.isPending} aria-busy={create.isPending} onClick={() => create.mutate({ scope, days })}>{t(($) => $.audit.export)}</Button>
    </div>
    {create.isError && <p role="alert" className="text-destructive">{create.error.message}</p>}
    {create.isSuccess && <p role="status"><AppLink href={paths.inbox()} className="text-primary hover:underline">{t(($) => $.audit.ready)}</AppLink></p>}
    {exportId && <div className="space-y-2 rounded-xl border p-4">
      <p className="text-caption text-muted-foreground">{t(($) => $.audit.download_help)}</p>
      <Button variant="outline" disabled={downloading} aria-busy={downloading} onClick={() => void download()}>{t(($) => $.audit.download)}</Button>
      {downloadError && <p role="alert" className="text-destructive">{downloadError}</p>}
    </div>}
    {records.isPending && <Skeleton className="h-48 w-full" />}
    {records.isError && <div role="alert"><p>{t(($) => $.audit.failed)}</p><Button variant="outline" onClick={() => void records.refetch()}>{t(($) => $.audit.retry)}</Button></div>}
    {records.isSuccess && records.data.pages[0]?.records.length === 0 && <p className="py-10 text-center text-muted-foreground">{t(($) => $.audit.empty)}</p>}
    <div className="overflow-x-auto">
      <table className="w-full text-left text-caption">
        <thead>
          <tr className="border-b">
            <th className="p-2">{t(($) => $.audit.time)}</th>
            <th className="p-2">{t(($) => $.audit.actor)}</th>
            <th className="p-2">{t(($) => $.audit.action)}</th>
            <th className="p-2">{t(($) => $.audit.details)}</th>
          </tr>
        </thead>
        <tbody>
          {records.data?.pages.flatMap((page) => page.records).map((record) => (
            <tr key={record.id} className="border-b align-top">
              <td className="whitespace-nowrap p-2">{new Date(record.created_at).toLocaleString(locale)}</td>
              <td className="p-2">
                {record.actor_name}
                <span className="block text-muted-foreground">{t(($) => $.audit[record.actor_type ?? "system"])}</span>
              </td>
              <td className="p-2">{actionLabels[record.action] ?? record.action}</td>
              <td className="max-w-lg p-2">
                <details>
                  <summary className="cursor-pointer">{t(($) => $.audit.details)}</summary>
                  <pre className="mt-2 max-h-72 overflow-auto whitespace-pre-wrap break-all">{JSON.stringify(record.details, null, 2)}</pre>
                </details>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
    {records.hasNextPage && <Button variant="outline" disabled={records.isFetchingNextPage} onClick={() => void records.fetchNextPage()}>{t(($) => $.audit.more)}</Button>}
  </section>;
}
