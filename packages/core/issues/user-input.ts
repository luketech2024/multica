import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type { UserInputResponse } from "./user-input-schema";

export function userInputOptions(wsId: string, issueId: string) {
  return queryOptions({
    queryKey: ["user-input", wsId, issueId],
    queryFn: () => api.listUserInputs(issueId),
    enabled: !!wsId && !!issueId,
    refetchInterval: 2000,
  });
}
export function useAnswerUserInput(wsId: string, issueId: string, requestId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (response: UserInputResponse) => api.answerUserInput(issueId, requestId, response),
    onSettled: () => client.invalidateQueries({ queryKey: userInputOptions(wsId, issueId).queryKey }),
  });
}
