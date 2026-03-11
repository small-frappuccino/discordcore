package control

import (
	"net/url"
	"strings"
)

type controlPublicOrigin struct {
	scheme string
	host   string
}

func (o controlPublicOrigin) valid() bool {
	return strings.TrimSpace(o.scheme) != "" && strings.TrimSpace(o.host) != ""
}

func (o controlPublicOrigin) resolve(target string) string {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" || !o.valid() {
		return trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	if parsed.IsAbs() {
		return parsed.String()
	}

	base := &url.URL{
		Scheme: o.scheme,
		Host:   o.host,
		Path:   "/",
	}
	return base.ResolveReference(parsed).String()
}

func controlPublicOriginFromAbsoluteURL(raw string) (controlPublicOrigin, bool) {
	parsed, err := parseAbsoluteURL(strings.TrimSpace(raw))
	if err != nil {
		return controlPublicOrigin{}, false
	}
	return controlPublicOrigin{
		scheme: parsed.Scheme,
		host:   parsed.Host,
	}, true
}

func (o *discordOAuthProvider) publicControlOrigin() controlPublicOrigin {
	if o == nil {
		return controlPublicOrigin{}
	}
	origin, ok := controlPublicOriginFromAbsoluteURL(o.redirectURI)
	if !ok {
		return controlPublicOrigin{}
	}
	return origin
}

func (s *Server) publicControlOrigin() controlPublicOrigin {
	if s == nil || s.discordOAuth == nil {
		return controlPublicOrigin{}
	}
	return s.discordOAuth.publicControlOrigin()
}

func (s *Server) publicControlURL(target string) string {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return ""
	}

	origin := s.publicControlOrigin()
	if !origin.valid() {
		return trimmed
	}
	return origin.resolve(trimmed)
}

func (s *Server) publicDashboardURL(target string) string {
	sanitized := sanitizeControlRedirectTarget(target)
	if sanitized == "" {
		sanitized = dashboardRoutePrefix
	}
	return s.publicControlURL(sanitized)
}

func (s *Server) publicDiscordOAuthLoginURL(next string) string {
	return s.publicControlURL(buildDiscordOAuthLoginPath(next))
}
