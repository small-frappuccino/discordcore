import { z } from "zod";

export const EmbedsSchema = z.object({
  webhook_url: z.string(),
  enabled: z.boolean(),
});

export type EmbedsFormData = z.infer<typeof EmbedsSchema>;
