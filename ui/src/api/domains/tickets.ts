import { z } from "zod";
import type { ControlApiClient } from "../client";

export const TicketIntakeQuestionSchema = z.object({
  id: z.string().uuid(),
  title: z.string().min(1, "Title is required").max(100),
  placeholder: z.string().max(200).optional(),
  required: z.boolean().default(true),
  multiline: z.boolean().default(false),
  minLength: z.number().min(0).optional(),
  maxLength: z.number().max(4000).optional(),
});

export type TicketIntakeQuestion = z.infer<typeof TicketIntakeQuestionSchema>;

export const TicketIntakeFormSchema = z.object({
  id: z.string().uuid(),
  name: z.string().min(1, "Form name is required").max(100),
  questions: z.array(TicketIntakeQuestionSchema),
});

export type TicketIntakeForm = z.infer<typeof TicketIntakeFormSchema>;

export const TicketPanelCategorySchema = z.object({
  id: z.string().uuid(),
  name: z.string().min(1, "Category name is required").max(100),
  description: z.string().max(200).optional(),
  emoji: z.string().optional(),
  formId: z.string().uuid().optional(),
  discordCategoryId: z.string().regex(/^\d{17,20}$/, "Invalid Discord Category ID"),
  staffRoleIds: z.array(z.string().regex(/^\d{17,20}$/, "Invalid Role ID")),
});

export type TicketPanelCategory = z.infer<typeof TicketPanelCategorySchema>;

export const TicketPanelSchema = z.object({
  id: z.string().uuid(),
  name: z.string().min(1, "Panel name is required").max(100),
  channelId: z.string().regex(/^\d{17,20}$/, "Invalid Discord Channel ID"),
  embedTitle: z.string().max(256),
  embedDescription: z.string().max(4000),
  embedColor: z.string().regex(/^#[0-9A-Fa-f]{6}$/, "Must be a valid hex color, e.g., #FFFFFF"),
  categories: z.array(TicketPanelCategorySchema).min(1, "At least one category is required"),
});

export type TicketPanel = z.infer<typeof TicketPanelSchema>;

export const TicketAutomationSettingsSchema = z.object({
  autoCloseTimerHours: z.number().min(0).max(720).optional(),
  inactivityWarningHours: z.number().min(0).max(720).optional(),
  transcriptChannelId: z.string().regex(/^\d{17,20}$/, "Invalid Channel ID").optional().or(z.literal("")),
});

export type TicketAutomationSettings = z.infer<typeof TicketAutomationSettingsSchema>;

export const TicketsFeatureConfigSchema = z.object({
  enabled: z.boolean().default(false),
  panels: z.array(TicketPanelSchema).default([]),
  forms: z.array(TicketIntakeFormSchema).default([]),
  automation: TicketAutomationSettingsSchema.default({}),
});

export type TicketsFeatureConfig = z.infer<typeof TicketsFeatureConfigSchema>;

// API Response Wrappers
export interface TicketsConfigResponse {
  status: string;
  guild_id: string;
  settings: TicketsFeatureConfig;
}

// Live Tickets & Transcripts Types
export interface LiveTicket {
  id: string;
  channel_id: string;
  panel_id: string;
  category_id: string;
  creator_id: string;
  creator_username: string;
  creator_avatar?: string;
  created_at: string;
  claimed_by_id?: string;
  claimed_by_username?: string;
}

export interface TicketMessage {
  id: string;
  author_id: string;
  author_username: string;
  author_avatar?: string;
  content: string;
  timestamp: string;
  attachments: { url: string; filename: string; size: number }[];
  is_staff: boolean;
}

export interface ClosedTicketTranscript {
  id: string;
  ticket_id: string;
  panel_name: string;
  category_name: string;
  creator_id: string;
  creator_username: string;
  closed_by_id: string;
  closed_by_username: string;
  created_at: string;
  closed_at: string;
  messages: TicketMessage[];
}

export interface LiveTicketsResponse {
  status: string;
  guild_id: string;
  tickets: LiveTicket[];
}

export interface TranscriptsListResponse {
  status: string;
  guild_id: string;
  transcripts: Omit<ClosedTicketTranscript, "messages">[];
}

export interface TranscriptDetailResponse {
  status: string;
  guild_id: string;
  transcript: ClosedTicketTranscript;
}

// API Functions
export async function getTicketsConfig(client: ControlApiClient, guildId: string): Promise<TicketsConfigResponse> {
  return client.request<TicketsConfigResponse>("GET", `/v1/guilds/${encodeURIComponent(guildId)}/tickets/settings`);
}

export async function updateTicketsConfig(
  client: ControlApiClient,
  guildId: string,
  payload: TicketsFeatureConfig,
): Promise<TicketsConfigResponse> {
  return client.request<TicketsConfigResponse>("PUT", `/v1/guilds/${encodeURIComponent(guildId)}/tickets/settings`, payload);
}

export async function getLiveTickets(client: ControlApiClient, guildId: string): Promise<LiveTicketsResponse> {
  return client.request<LiveTicketsResponse>("GET", `/v1/guilds/${encodeURIComponent(guildId)}/tickets/live`);
}

export async function getTranscriptsList(client: ControlApiClient, guildId: string): Promise<TranscriptsListResponse> {
  return client.request<TranscriptsListResponse>("GET", `/v1/guilds/${encodeURIComponent(guildId)}/tickets/transcripts`);
}

export async function getTranscriptDetail(
  client: ControlApiClient,
  guildId: string,
  transcriptId: string,
): Promise<TranscriptDetailResponse> {
  return client.request<TranscriptDetailResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/tickets/transcripts/${encodeURIComponent(transcriptId)}`,
  );
}
