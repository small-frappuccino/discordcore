import { z } from "zod";

export const CustomEmbedFieldSchema = z.object({
  name: z.string().min(1, "Name is required"),
  value: z.string().min(1, "Value is required"),
  inline: z.boolean().optional(),
});

export const CustomEmbedPostingSchema = z.object({
  channel_id: z.string(),
  message_id: z.string().optional(),
  webhook_url: z.string().optional(),
});

export const EmbedsSchema = z.object({
  key: z.string().min(1, "Key is required"),
  title: z.string().optional(),
  description: z.string().optional(),
  color: z.number().optional(),
  author_name: z.string().optional(),
  author_icon_url: z.string().optional(),
  footer_text: z.string().optional(),
  footer_icon_url: z.string().optional(),
  image_url: z.string().optional(),
  thumbnail_url: z.string().optional(),
  fields: z.array(CustomEmbedFieldSchema).optional(),
  postings: z.array(CustomEmbedPostingSchema).optional(),
});

export type EmbedsFormData = z.infer<typeof EmbedsSchema>;
