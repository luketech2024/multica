"use client";

import { useEffect, useState } from "react";
import { useQueries } from "@tanstack/react-query";
import type { IssueTableQuerySpec, WorkbenchBandFilter } from "@multica/core/types";
import { issueTableFacetsOptions } from "@multica/core/issues/queries";
import { workbenchBandQuery, workbenchBandSelection } from "@multica/core/workbench/status-band";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
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
    queries: bands.map((band) => ({
      ...issueTableFacetsOptions(workspaceId, {
        query: workbenchBandQuery(query, { ...selection, band }),
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
      className="flex shrink-0 flex-wrap items-center gap-2 border-b px-4 py-2"
    >
      <Button
        variant={selection.band === "all" ? "secondary" : "ghost"}
        size="sm"
        aria-pressed={selection.band === "all"}
        onClick={() => onSelect("all", selection.only_mine)}
      >
        {t(($) => $.workbench_band.all)}
      </Button>
      {bands.map((band, index) => (
        <Button
          key={band}
          variant={selection.band === band ? "secondary" : "ghost"}
          size="sm"
          aria-pressed={selection.band === band}
          onClick={() => onSelect(band, selection.only_mine)}
        >
          {t(($) => $.workbench_band[band])}
          <span className="tabular-nums">
            {enabled && !counts[index]?.isError ? counts[index]?.data?.total ?? "—" : "—"}
          </span>
        </Button>
      ))}
      <label className="ml-auto flex items-center gap-2 text-caption">
        <Checkbox
          checked={selection.only_mine}
          onCheckedChange={(checked) => onSelect(selection.band, checked === true)}
        />
        {t(($) => $.workbench_band.mine)}
      </label>
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
