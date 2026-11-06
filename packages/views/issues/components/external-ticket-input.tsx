"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { externalTicketDuplicatesOptions } from "@multica/core/issues";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";

export function ExternalTicketInput({
  value,
  onChange,
  issueId,
  disabled = false,
}: {
  value: string;
  onChange: (value: string) => void;
  issueId?: string;
  disabled?: boolean;
}) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const [checkedValue, setCheckedValue] = useState("");
  const duplicates = useQuery(
    externalTicketDuplicatesOptions(wsId, checkedValue, issueId),
  );
  return (
    <label className="block space-y-1 text-caption">
      {t(($) => $.ticket.external)}
      <Input
        maxLength={32}
        pattern="[A-Za-z0-9-]*"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        onBlur={() => setCheckedValue(value)}
      />
      {value === checkedValue && (duplicates.data?.count ?? 0) > 0 && (
        <span className="block text-warning">
          {t(($) => $.ticket.duplicate)}
        </span>
      )}
    </label>
  );
}
