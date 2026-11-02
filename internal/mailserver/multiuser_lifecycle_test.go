package mailserver

import "testing"

type sequenceCloser struct {
	name   string
	closed *[]string
}

func (c sequenceCloser) Close() error { *c.closed = append(*c.closed, c.name); return nil }
func TestIsolatedShutdownStopsConsumersBeforeAuthority(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		server, err := NewMailServerWithOptions(0, "localhost", t.TempDir(), ServerOptions{RequireOwnership: isolated})
		if err != nil {
			t.Fatal(err)
		}
		if isolated && server.RelayService() != nil {
			t.Fatal("isolated storage initialized a global relay owner")
		}
		closed := []string{}
		for _, name := range []string{"authority", "api"} {
			if err = server.AddCloser(sequenceCloser{name, &closed}); err != nil {
				t.Fatal(err)
			}
		}
		if err = server.Close(); err != nil {
			t.Fatal(err)
		}
		wantFirst := "authority"
		if isolated {
			wantFirst = "api"
		}
		if len(closed) != 2 || closed[0] != wantFirst {
			t.Fatalf("isolated=%t order=%v", isolated, closed)
		}
	}
}
