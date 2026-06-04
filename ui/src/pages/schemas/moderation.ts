import { z } from "zod";

export const ModerationSchema = z.object({
  mute_role: z.string(),
});

export type ModerationFormData = z.infer<typeof ModerationSchema>;
