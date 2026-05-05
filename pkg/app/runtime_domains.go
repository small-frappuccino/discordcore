package app

import "github.com/small-frappuccino/discordcore/pkg/files"

const runtimeDefaultDomain = ""

type runtimeDomainSupport struct {
	all     bool
	domains map[string]struct{}
}

func newRuntimeDomainSupport(rawDomains []string) runtimeDomainSupport {
	if len(rawDomains) == 0 {
		return runtimeDomainSupport{all: true}
	}

	domains := make(map[string]struct{}, len(rawDomains))
	for _, rawDomain := range rawDomains {
		domains[normalizeSupportedRuntimeDomain(rawDomain)] = struct{}{}
	}

	return runtimeDomainSupport{domains: domains}
}

func (s runtimeDomainSupport) supportsDefaultDomain() bool {
	return s.supports(runtimeDefaultDomain)
}

func (s runtimeDomainSupport) supports(domain string) bool {
	if s.all {
		return true
	}

	_, ok := s.domains[normalizeSupportedRuntimeDomain(domain)]
	return ok
}

func normalizeSupportedRuntimeDomain(domain string) string {
	normalized := files.NormalizeBotDomain(domain)
	switch normalized {
	case "", "core", "default":
		return runtimeDefaultDomain
	default:
		return normalized
	}
}
