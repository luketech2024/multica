"use client";

import { useState } from "react";
import {
  useConfirmReleaseOrder,
  type ReleaseOrder,
} from "@multica/core/workbench/releases";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";

export function ReleaseGates({
  wsId,
  order,
}: {
  wsId: string;
  order: ReleaseOrder;
}) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const confirm = useConfirmReleaseOrder(wsId, order.id);
  const [changes, setChanges] = useState(order.confirmations.changes);
  const [migration, setMigration] = useState(order.confirmations.migration);
  const [waiver, setWaiver] = useState(order.confirmations.security_waiver);
  const editable =
    (order.env === "prod" || Boolean(order.manifest.delivery)) &&
    ["draft", "prechecking", "ready"].includes(order.status);
  const waiverRequired =
    order.gates?.some((gate) =>
      gate.blockers.some((blocker) => blocker.code === "security_waiver"),
    ) || order.confirmations.security_waiver;
  const reasons = t(($) => $.releases.gates.reasons, { returnObjects: true });
  return (
    <section className="space-y-3 rounded-xl border p-4">
      <h2 className="text-title-sm font-medium">
        {t(($) => $.releases.gates.title)}
      </h2>
      {!order.gates && (
        <p role="status">{t(($) => $.releases.gates.unavailable)}</p>
      )}
      <div className="grid gap-3 md:grid-cols-2">
        {order.gates?.map((gate) => (
          <div key={gate.id} className="space-y-1">
            <h3 className="text-caption font-medium">
              {order.manifest.delivery && gate.id === "targets"
                ? t(($) => $.application_deployment.configuration_gate)
                : t(($) => $.releases.gates.names[gate.id])}{" "}
              ·{" "}
              {!gate.required
                ? order.manifest.delivery && gate.id === "acceptance"
                  ? t(($) => $.application_deployment.existing_source_gate)
                  : t(($) => $.releases.gates.not_required)
                : gate.passed
                  ? t(($) => $.releases.gates.passed)
                  : t(($) => $.releases.gates.blocked)}
            </h3>
            <ul className="space-y-1 text-caption text-muted-foreground">
              {gate.blockers.map((blocker, index) => (
                <li key={`${blocker.code}-${index}`} className="break-all">
                  {reasons[blocker.code as keyof typeof reasons] ??
                    t(($) => $.releases.gates.unavailable)}
                  {blocker.resource_id && <span> · {blocker.resource_id}</span>}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      {order.gates?.some((gate) => gate.id === "targets" && !gate.passed) && (
        <AppLink
          className="text-caption text-brand underline"
          href={`${paths.projectDetail(order.project_id)}?tab=deployment`}
        >
          {t(($) => $.deployment.title)}
        </AppLink>
      )}
      {editable && (
        <fieldset
          disabled={confirm.isPending}
          className="space-y-2 text-caption"
        >
          <label className="flex items-start gap-2">
            <input
              type="checkbox"
              checked={changes}
              onChange={(event) => setChanges(event.target.checked)}
            />
            {t(($) => $.releases.gates.confirm_changes)}
          </label>
          <label className="flex items-start gap-2">
            <input
              type="checkbox"
              checked={migration}
              onChange={(event) => setMigration(event.target.checked)}
            />
            {t(($) => $.releases.gates.confirm_migration)}
          </label>
          {waiverRequired && (
            <label className="flex items-start gap-2">
              <input
                type="checkbox"
                checked={waiver}
                onChange={(event) => setWaiver(event.target.checked)}
              />
              {t(($) => $.releases.gates.confirm_waiver)}
            </label>
          )}
          <Button
            disabled={confirm.isPending}
            onClick={() =>
              confirm.mutate({
                expected_revision: order.revision,
                changes,
                migration,
                security_waiver: waiver,
              })
            }
          >
            {t(($) => $.releases.gates.save)}
          </Button>
          {confirm.isError && (
            <p role="alert" className="text-destructive">
              {confirm.error.message}
            </p>
          )}
        </fieldset>
      )}
    </section>
  );
}
