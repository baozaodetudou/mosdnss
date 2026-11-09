package adguard_rule

import (
	"strings"
	"testing"

	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
)

func TestParseSimpleDomainRule(t *testing.T) {
	tests := []struct {
		line  string
		want  string
		allow bool
		ok    bool
	}{
		{line: "||ads.example.com^", want: "ads.example.com", ok: true},
		{line: "@@||ads.example.com^", want: "ads.example.com", allow: true, ok: true},
		{line: "||*.ads.example.com^", want: "*.ads.example.com", ok: true},
		{line: "||ads.example.com^$third-party", ok: false},
		{line: "example.com", ok: false},
	}

	for _, tt := range tests {
		got, allow, ok := parseSimpleDomainRule(tt.line)
		if got != tt.want || allow != tt.allow || ok != tt.ok {
			t.Fatalf("parseSimpleDomainRule(%q) = (%q, %t, %t), want (%q, %t, %t)", tt.line, got, allow, ok, tt.want, tt.allow, tt.ok)
		}
	}
}

func TestParseRulesKeepsAllowAndDenySemantics(t *testing.T) {
	allow := domain.NewDomainMixMatcher()
	deny := domain.NewDomainMixMatcher()
	input := strings.NewReader("! comment\n||ads.example.com^\n@@||allowed.ads.example.com^\nexample.net\n")

	count, err := parseRules(input, allow, deny)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("parsed rule count = %d, want 3", count)
	}
	if _, ok := allow.Match("allowed.ads.example.com"); !ok {
		t.Fatal("allow rule did not match")
	}
	if _, ok := deny.Match("ads.example.com"); !ok {
		t.Fatal("domain block rule did not match")
	}
	if _, ok := deny.Match("example.net"); !ok {
		t.Fatal("full domain rule did not match")
	}
}
