package control

import (
	"fmt"
	"net"
	"net/http"
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

func (o controlPublicOrigin) string() string {
	if !o.valid() {
		return ""
	}
	return o.scheme + "://" + o.host
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
	if s == nil {
		return controlPublicOrigin{}
	}
	if s.publicOrigin.valid() {
		return s.publicOrigin
	}
	if s.discordOAuth == nil {
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

func (s *Server) SetPublicOrigin(raw string) error {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		s.publicOrigin = controlPublicOrigin{}
		return nil
	}
	origin, ok := controlPublicOriginFromAbsoluteURL(trimmed)
	if !ok {
		return fmt.Errorf("invalid control public origin %q", trimmed)
	}
	s.publicOrigin = origin
	return nil
}

func (s *Server) canonicalPublicRedirectURL(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", false
	}
	origin := s.publicOrigin
	if !origin.valid() || origin.matchesRequest(r) {
		return "", false
	}
	target := r.URL.Path
	if strings.TrimSpace(r.URL.RawQuery) != "" {
		target += "?" + r.URL.RawQuery
	}
	return origin.resolve(target), true
}

func (o controlPublicOrigin) matchesRequest(r *http.Request) bool {
	if !o.valid() || r == nil {
		return true
	}
	return strings.EqualFold(o.scheme, requestScheme(r)) &&
		strings.EqualFold(normalizePublicHost(o.host, o.scheme), normalizePublicHost(requestHost(r), requestScheme(r)))
}

func requestScheme(r *http.Request) string {
	if r == nil {
		return ""
	}
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if forwarded != "" {
		if comma := strings.Index(forwarded, ","); comma >= 0 {
			forwarded = strings.TrimSpace(forwarded[:comma])
		}
		return strings.ToLower(forwarded)
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if host := strings.TrimSpace(r.Host); host != "" {
		return host
	}
	return strings.TrimSpace(r.URL.Host)
}

func normalizePublicHost(raw string, scheme string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		return trimmed
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return strings.ToLower(host)
	}
	return strings.ToLower(net.JoinHostPort(host, port))
}
