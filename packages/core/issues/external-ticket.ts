import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export function externalTicketDuplicatesOptions(
  wsId: string,
  value: string,
  excludeId?: string,
) {
  return queryOptions({
    queryKey: ["issues", wsId, "external-ticket-duplicates", value, excludeId],
    queryFn: () => api.countExternalTicketDuplicates(value, excludeId),
    enabled: !!value && /^[A-Za-z0-9-]{1,32}$/.test(value),
    staleTime: 0,
  });
}
