export interface DiscordOAuthUser {
  id: string;
  username: string;
  discriminator?: string;
  global_name?: string;
  avatar?: string;
}

export interface AuthSessionResponse {
  status: string;
  user: DiscordOAuthUser;
  scopes: string[];
  csrf_token: string;
  expires_at: string;
}

export interface DiscordOAuthStatusResponse {
  status: string;
  oauth_configured: boolean;
  authenticated: boolean;
  dashboard_url: string;
  login_url: string;
  user?: DiscordOAuthUser;
  scopes?: string[];
  csrf_token?: string;
  expires_at?: string;
}

export type ControlAuthProbe =
  | { status: "authenticated"; session: AuthSessionResponse }
  | { status: "unauthorized" }
  | { status: "oauth_unavailable" };
