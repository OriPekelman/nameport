package policy

import (
	"testing"
)

func TestNewPolicy(t *testing.T) {
	p := NewPolicy()
	if p == nil {
		t.Fatal("NewPolicy() returned nil")
	}
	if len(p.allowedTLDs) == 0 {
		t.Fatal("allowedTLDs is empty")
	}
	if len(p.blockedTLDs) == 0 {
		t.Fatal("blockedTLDs is empty; IANA TLD list was not loaded")
	}
}

func TestIsAllowedTLD(t *testing.T) {
	p := NewPolicy()

	allowed := []string{".localhost", ".test", ".localdev", ".internal", ".home.arpa"}
	for _, tld := range allowed {
		if !p.IsAllowedTLD(tld) {
			t.Errorf("IsAllowedTLD(%q) = false, want true", tld)
		}
	}

	// Case insensitive.
	if !p.IsAllowedTLD(".LOCALHOST") {
		t.Error("IsAllowedTLD(.LOCALHOST) = false, want true (case insensitive)")
	}

	notAllowed := []string{".com", ".org", ".net", ".io", ".dev", ".xyz"}
	for _, tld := range notAllowed {
		if p.IsAllowedTLD(tld) {
			t.Errorf("IsAllowedTLD(%q) = true, want false", tld)
		}
	}
}

func TestValidateDomain_Allowed(t *testing.T) {
	p := NewPolicy()

	cases := []string{
		"myapp.localhost",
		"deep.sub.myapp.localhost",
		"myapp.test",
		"myapp.localdev",
		"myapp.internal",
		"myapp.home.arpa",
		"sub.myapp.home.arpa",
		"MYAPP.LOCALHOST",
		"myapp.localhost.", // trailing dot
	}
	for _, domain := range cases {
		if err := p.ValidateDomain(domain); err != nil {
			t.Errorf("ValidateDomain(%q) = %v, want nil", domain, err)
		}
	}
}

func TestValidateDomain_Blocked(t *testing.T) {
	p := NewPolicy()

	cases := []string{
		"example.com",
		"example.org",
		"example.net",
		"evil.io",
		"phishing.dev",
		"something.xyz",
		"google.com",
		"sub.domain.co.uk",
	}
	for _, domain := range cases {
		if err := p.ValidateDomain(domain); err == nil {
			t.Errorf("ValidateDomain(%q) = nil, want error", domain)
		}
	}
}

func TestValidateDomain_EdgeCases(t *testing.T) {
	p := NewPolicy()

	// Empty domain.
	if err := p.ValidateDomain(""); err == nil {
		t.Error("ValidateDomain(\"\") = nil, want error")
	}

	// Whitespace only.
	if err := p.ValidateDomain("  "); err == nil {
		t.Error("ValidateDomain(\"  \") = nil, want error")
	}

	// Just a TLD (allowed).
	if err := p.ValidateDomain("localhost"); err != nil {
		t.Errorf("ValidateDomain(\"localhost\") = %v, want nil", err)
	}

	// Just a TLD (blocked).
	if err := p.ValidateDomain("com"); err == nil {
		t.Error("ValidateDomain(\"com\") = nil, want error")
	}
}

func TestValidateWildcard_Allowed(t *testing.T) {
	p := NewPolicy()

	cases := []string{
		"*.myapp.localhost",
		"*.deep.myapp.localhost",
		"*.myapp.test",
		"*.myapp.localdev",
		"*.myapp.internal",
		"*.myapp.home.arpa",
		"*.MYAPP.LOCALHOST",
	}
	for _, pat := range cases {
		if err := p.ValidateWildcard(pat); err != nil {
			t.Errorf("ValidateWildcard(%q) = %v, want nil", pat, err)
		}
	}
}

func TestValidateWildcard_Blocked(t *testing.T) {
	p := NewPolicy()

	cases := []string{
		"*.example.com",
		"*.evil.io",
		"*.google.dev",
	}
	for _, pat := range cases {
		if err := p.ValidateWildcard(pat); err == nil {
			t.Errorf("ValidateWildcard(%q) = nil, want error", pat)
		}
	}
}

func TestValidateWildcard_Invalid(t *testing.T) {
	p := NewPolicy()

	cases := []struct {
		pattern string
		desc    string
	}{
		{"*.localhost", "wildcard directly under TLD (depth < 2)"},
		{"*.test", "wildcard directly under TLD (depth < 2)"},
		{"*.home.arpa", "wildcard directly under multi-label TLD (depth < 2)"},
		{"myapp.*.localhost", "wildcard not in left-most position"},
		{"**.myapp.localhost", "double star"},
		{"*.*.myapp.localhost", "multiple wildcards"},
		{"", "empty pattern"},
		{"*", "bare wildcard"},
	}
	for _, tc := range cases {
		if err := p.ValidateWildcard(tc.pattern); err == nil {
			t.Errorf("ValidateWildcard(%q) [%s] = nil, want error", tc.pattern, tc.desc)
		}
	}
}

func TestBlockedTLDsContainCommonEntries(t *testing.T) {
	p := NewPolicy()

	// Sanity-check that common TLDs were loaded from the embedded file.
	common := []string{".com", ".net", ".org", ".io", ".dev", ".app", ".uk", ".de", ".fr", ".jp"}
	for _, tld := range common {
		if !p.blockedTLDs[tld] {
			t.Errorf("blockedTLDs missing %q", tld)
		}
	}
}
