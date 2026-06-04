import { z } from "zod";

export const PartnersSchema = z.object({
  title: z.string().optional(),
  continuation_title: z.string().optional(),
  intro: z.string().optional(),
  section_header_template: z.string().optional(),
  section_continuation_suffix: z.string().optional(),
  section_continuation_template: z.string().optional(),
  line_template: z.string().optional(),
  empty_state_text: z.string().optional(),
  footer_template: z.string().optional(),
  other_fandom_label: z.string().optional(),
  color: z.number().optional(),
  disable_fandom_sorting: z.boolean().optional(),
  disable_partner_sorting: z.boolean().optional(),
});

export type PartnersFormData = z.infer<typeof PartnersSchema>;
