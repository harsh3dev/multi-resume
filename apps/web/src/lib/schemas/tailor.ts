import { z } from "zod";

// Schema for the streamed tailor object (BUILD-PLAN §9.3).
export const tailorChangesSchema = z.object({
  changes: z.array(
    z.object({
      op: z.enum(["edit", "hide", "show"]),
      id: z.string(),
      value: z.string().optional(),
      why: z.string(),
      flags: z.array(z.string()),
    }),
  ),
  done: z.boolean().optional(),
  runId: z.string().optional(),
  error: z.boolean().optional(),
});

export type TailorChange = z.infer<typeof tailorChangesSchema>["changes"][number];
