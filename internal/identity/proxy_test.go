package identity

import (
	"net/http"
	"testing"
)

func TestProxyTrustAndEvidence(t *testing.T) {
	p, err := NewProxy("main", []string{"127.0.0.1/32"}, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
	if err != nil {
		t.Fatal(err)
	}
	good := http.Header{"X-Auth-User": {"a"}, "X-Auth-Email": {"a@example.com"}, "X-Auth-Email-Verified": {"true"}}
	a, err := p.Verify("127.0.0.1:1234", good)
	if err != nil || !a.EmailVerified || a.Subject != "a" {
		t.Fatal(a, err)
	}
	for _, peer := range []string{"192.0.2.1:1234", "127.0.0.1", "invalid"} {
		if _, err = p.Verify(peer, good); err == nil {
			t.Errorf("trusted %q", peer)
		}
	}
	for _, h := range []http.Header{
		{}, {"X-Auth-User": {"a", "b"}}, {"X-Auth-User": {"a"}, "x-auth-user": {"b"}},
		{"X-Auth-User": {"a"}, "X-Auth-Email-Verified": {"true"}},
		{"X-Auth-User": {"a"}, "X-Auth-Email-Verified": {"yes"}},
	} {
		if _, err = p.Verify("127.0.0.1:1", h); err == nil {
			t.Fatal("accepted invalid assertion")
		}
	}
	noEvidence := good.Clone()
	noEvidence.Del("X-Auth-Email-Verified")
	a, err = p.Verify("127.0.0.1:1", noEvidence)
	if err != nil || a.EmailVerified {
		t.Fatal("invented verification")
	}
	spoof := good.Clone()
	spoof.Set("X-Forwarded-For", "127.0.0.1")
	if _, err = p.Verify("192.0.2.1:1", spoof); err == nil {
		t.Fatal("trusted forwarded header")
	}
}
