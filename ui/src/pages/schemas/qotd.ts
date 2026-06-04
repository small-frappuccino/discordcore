import { z } from "zod";

export const QOTDSchema = z.object({
  verified_role_id: z.string().optional(),
  active_deck_id: z.string().optional(),
  schedule: z.object({
    hour_utc: z.number().optional(),
    minute_utc: z.number().optional(),
  }).optional(),
});

export type QOTDFormData = z.infer<typeof QOTDSchema>;
