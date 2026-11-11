"use client";

import { useEffect, useState } from "react";
import { useQueries } from "@tanstack/react-query";
import type { IssueTableQuerySpec, WorkbenchBandFilter } from "@multica/core/types";
import { issueTableFacetsOptions } from "@multica/core/issues/queries";
import { workbenchBandQuery, workbenchBandSelection } from "@multica/core/workbench/status-band";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Check, ChevronRight, BarChart3 } from "@multica/ui/icons";
import { cn } from "@multica/ui/lib/utils";
import { useOptionalNavigation } from "../../navigation";
import { useT } from "../../i18n";

export function useWorkbenchBand(enabled: boolean, defaultMine: boolean) {
  const navigation = useOptionalNavigation();
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    if (!enabled) return;
    const timer = setInterval(() => setNow(new Date()), 60000);
    return () => clearInterval(timer);
  }, [enabled]);
  const params = navigation?.searchParams;
  useEffect(() => {
    if (!enabled || !defaultMine || !navigation || params?.has("wb_mine")) return;
    const next = new URLSearchParams(params);
    next.set("wb_mine", "1");
    navigation.replace(`${navigation.pathname}?${next}`);
  }, [defaultMine, enabled, navigation, params]);
  const selection = enabled && navigation
    ? workbenchBandSelection(navigation.searchParams, defaultMine, now)
    : undefined;
  const select = (band: WorkbenchBandFilter["band"], mine: boolean) => {
    if (!navigation) return;
    const next = new URLSearchParams(navigation.searchParams);
    next.set("wb_band", band);
    next.set("wb_mine", mine ? "1" : "0");
    navigation.replace(`${navigation.pathname}?${next}`);
  };
  return { selection, select, clear: () => select("all", false) };
}

const bands = ["running", "review", "today"] as const;
const bandIcons = [ChevronRight, Check, BarChart3];
const bandColors = ["bg-brand/14 text-brand", "bg-warning/14 text-warning", "bg-success/14 text-success"];

export function WorkbenchStatusBand({
  workspaceId,
  query,
  selection,
  onSelect,
  enabled,
}: {
  workspaceId: string;
  query: IssueTableQuerySpec;
  selection: WorkbenchBandFilter;
  onSelect: (band: WorkbenchBandFilter["band"], mine: boolean) => void;
  enabled: boolean;
}) {
  const { t } = useT("issues");
  const counts = useQueries({
    queries: [...bands, "all" as const].map((band) => ({
      ...issueTableFacetsOptions(workspaceId, {
        query: workbenchBandQuery(query, { ...selection, band, only_mine: band === "all" ? false : selection.only_mine }),
        facets: [{ kind: "status" }],
        include_total: true,
      }),
      enabled,
      refetchInterval: 30000,
    })),
  });

  return (
    <section
      aria-label={t(($) => $.workbench_band.title)}
      className="flex shrink-0 flex-wrap items-center gap-3 px-6 py-4"
    >
      {bands.map((band, index) => {
        const Icon = bandIcons[index]!;
        return <button
          type="button"
          key={band}
          className={cn("flex min-w-[132px] items-center gap-3 rounded-xl border border-surface-border bg-surface px-3 py-3 text-left shadow-[var(--surface-shadow)] transition-[box-shadow,transform] hover:shadow-[var(--surface-shadow-hover)] motion-safe:hover:-translate-y-px focus-visible:ring-2 focus-visible:ring-ring", selection.band === band && "border-brand/55 bg-brand/8")}
          aria-pressed={selection.band === band}
          onClick={() => onSelect(selection.band === band ? "all" : band, selection.only_mine)}
        >
          <span className={cn("grid size-7 shrink-0 place-items-center rounded-lg", bandColors[index])}><Icon className="size-4" /></span>
          <span className="flex flex-col">
            <span className="text-display-sm font-medium leading-6 tabular-nums">{enabled && !counts[index]?.isError ? counts[index]?.data?.total ?? "—" : "—"}</span>
            <span className="text-caption text-muted-foreground">{t(($) => $.workbench_band[band])}</span>
          </span>
        </button>;
      })}
      <div className="ml-auto flex items-center gap-1 rounded-lg border border-surface-border bg-surface p-1">
      <label className={cn("flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1 text-caption", selection.only_mine && "bg-surface-hover")}>
        <Checkbox
          checked={selection.only_mine}
          onCheckedChange={(checked) => onSelect(selection.band, checked === true)}
        />
        {t(($) => $.workbench_band.mine)}
      </label>
      <Button variant="ghost" size="sm" aria-pressed={!selection.only_mine && selection.band === "all"} onClick={() => onSelect("all", false)}>{t(($) => $.workbench_band.all)}</Button>
      </div>
      <div className="flex w-full flex-wrap items-center gap-x-4 gap-y-1 text-label text-muted-foreground">
        <span>{t(($) => $.workbench_ui.view_total, { total: enabled && !counts[3]?.isError ? counts[3]?.data?.total ?? "—" : "—" })}</span>
        {selection.only_mine && <span>{t(($) => $.workbench_ui.mine_hint)}</span>}
        {selection.band !== "all" && <span>{t(($) => $.workbench_ui.band_hint, { band: t(($) => $.workbench_band[selection.band as typeof bands[number]]) })}</span>}
      </div>
      {counts.some((count) => count.isError) && (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => { for (const count of counts) void count.refetch(); }}
        >
          {t(($) => $.workbench_band.retry)}
        </Button>
      )}
    </section>
  );
}
