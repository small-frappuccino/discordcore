export interface DiscordUser {
  id: string;
  username: string;
  avatar: string;
}

export interface DiscordEmbed {
  title?: string;
  description?: string;
  color?: number;
}

export interface DiscordComponent {
  type: number;
  components?: DiscordComponent[];
  label?: string;
  style?: number;
  custom_id?: string;
}

export interface DiscordMessage {
  id: string;
  author: DiscordUser;
  content: string;
  timestamp: string;
  embeds?: DiscordEmbed[];
  components?: DiscordComponent[];
}
