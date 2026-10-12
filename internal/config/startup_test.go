package config

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestParseFlagsRejectsUnconsumedArguments(t *testing.T) {
	t.Setenv("OWLMAIL_CONFIG_FILE", "")
	t.Setenv("OWLMAIL_SMTP_HOST", "127.0.0.1")
	for _, test := range []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"defaults", nil, false},
		{"empty flag value", []string{"-disable-web", "-ip", ""}, false},
		{"empty positional argument", []string{""}, true},
		{"trailing empty positional argument", []string{"-disable-web", ""}, true},
		{"nonempty positional argument", []string{"unexpected"}, true},
		{"argument after separator", []string{"-disable-web", "--", ""}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalArgs, originalFlags := os.Args, flag.CommandLine
			t.Cleanup(func() { os.Args, flag.CommandLine = originalArgs, originalFlags })
			os.Args = append([]string{"owlmail"}, test.args...)
			flag.CommandLine = flag.NewFlagSet("owlmail", flag.ContinueOnError)
			cfg, err := ParseFlags()
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "does not accept positional arguments") {
					t.Fatalf("unconsumed arguments were not rejected: config=%+v err=%v", cfg, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(test.args) > 0 && (!cfg.DisableWeb || cfg.SMTPHost != "") {
				t.Fatalf("explicit empty flag value was not preserved: disable-web=%t ip=%q", cfg.DisableWeb, cfg.SMTPHost)
			}
		})
	}
}
