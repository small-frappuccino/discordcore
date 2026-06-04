import { z } from "zod";

export const RolesSchema = z.object({
  dashboard_read: z.array(z.string()),
  dashboard_write: z.array(z.string()),
  booster_role: z.string(),
  mute_role: z.string(),
  auto_assignment: z.object({
    enabled: z.boolean(),
    target_role: z.string(),
    required_roles: z.array(z.string()),
  })
});

export type RolesFormData = z.infer<typeof RolesSchema>;
