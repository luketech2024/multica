import { z } from "zod";
import { parseWithFallback } from "../api/schema";

export const UserInputQuestionSchema = z.object({
  id: z.string(), header: z.string(), question: z.string(),
  isOther: z.boolean().default(false), isSecret: z.boolean().default(false),
  options: z.array(z.object({ label: z.string(), description: z.string(), recommended: z.boolean().optional() })).nullish().transform((value) => value ?? []),
});
export const UserInputResponseSchema = z.object({ discussion_comment_id: z.string().optional(), answers: z.record(z.string(), z.object({ answers: z.array(z.string()) })) });
export const TaskUserInputSchema = z.object({
  id: z.string(), task_id: z.string(), issue_id: z.string(), status: z.string(), created_at: z.string(),
  answered_by: z.string().optional(),
  request: z.object({ origin: z.string().optional(), key: z.string(), threadId: z.string().default(""), turnId: z.string().default(""), questions: z.array(UserInputQuestionSchema).min(1).max(3) }),
  response: UserInputResponseSchema.nullable(),
});
export type TaskUserInput = z.infer<typeof TaskUserInputSchema>;
export type UserInputResponse = z.infer<typeof UserInputResponseSchema>;
export function parseTaskUserInputs(raw: unknown): TaskUserInput[] {
  return parseWithFallback<TaskUserInput[]>(raw, z.array(TaskUserInputSchema), [], { endpoint: "GET /api/issues/:id/input-requests" });
}
