"use client";

import { useState } from "react";
import type { Issue } from "@multica/core/types";
import { useUpdateIssue } from "@multica/core/issues/mutations";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";
import { ExternalTicketInput } from "./external-ticket-input";

export function ExternalTicketEditor({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const update = useUpdateIssue();
  const [draft, setDraft] = useState<{ value: string; revision: number }>();
  const [error, setError] = useState<string>();

  const save = async () => {
    if (!draft) return;
    setError(undefined);
    try {
      await update.mutateAsync({
        id: issue.id,
        expected_revision: draft.revision,
        external_ticket_no: draft.value,
      });
      setDraft(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t(($) => $.ticket.save_failed),
      );
    }
  };

  return (
    <>
      <Button
        size="xs"
        variant="outline"
        disabled={issue.revision === undefined}
        aria-label={t(($) => $.ticket.edit)}
        onClick={() => {
          setError(undefined);
          setDraft({
            value: issue.external_ticket_no ?? "",
            revision: issue.revision!,
          });
        }}
      >
        {issue.external_ticket_no
          ? `#${issue.external_ticket_no}`
          : t(($) => $.ticket.add)}
      </Button>
      <Dialog
        open={!!draft}
        onOpenChange={(open) => {
          if (!open && !update.isPending) setDraft(undefined);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(($) => $.ticket.edit)}</DialogTitle>
          </DialogHeader>
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <label className="block space-y-1 text-caption">
              {t(($) => $.ticket.system)}
              <Input readOnly value={issue.identifier} />
            </label>
            <ExternalTicketInput
              value={draft?.value ?? ""}
              issueId={issue.id}
              disabled={update.isPending}
              onChange={(value) =>
                setDraft((current) => current && { ...current, value })
              }
            />
            {error && (
              <p role="alert" className="text-caption text-destructive">
                {error}
              </p>
            )}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                disabled={update.isPending}
                onClick={() => setDraft(undefined)}
              >
                {t(($) => $.requirements.discard)}
              </Button>
              <Button
                type="submit"
                disabled={update.isPending}
                aria-busy={update.isPending}
              >
                {t(($) => $.requirements.save)}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
