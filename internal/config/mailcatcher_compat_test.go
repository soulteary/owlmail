package config

import (
	"flag"
	"testing"
)

func TestMailCatcherRESTCompatConfiguration(t *testing.T) {
	fs := flag.NewFlagSet("default", flag.ContinueOnError)
	refs := DefineFlags(fs)
	_ = fs.Parse(nil)
	if ResolveConfig(fs, refs).MailCatcherRESTCompat {
		t.Fatal("MailCatcher facade must be disabled by default")
	}

	t.Setenv("OWLMAIL_MAILCATCHER_REST_COMPAT", "true")
	fs = flag.NewFlagSet("environment", flag.ContinueOnError)
	refs = DefineFlags(fs)
	_ = fs.Parse(nil)
	if !ResolveConfig(fs, refs).MailCatcherRESTCompat {
		t.Fatal("environment did not enable MailCatcher facade")
	}

	fs = flag.NewFlagSet("cli", flag.ContinueOnError)
	refs = DefineFlags(fs)
	_ = fs.Parse([]string{"-mailcatcher-rest-compat=false"})
	if ResolveConfig(fs, refs).MailCatcherRESTCompat {
		t.Fatal("CLI did not override the environment")
	}
}
