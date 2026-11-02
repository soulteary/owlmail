package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"testing"
)

func TestMultiuserDoctorReadsAuthorityWithoutMutation(t *testing.T) {
	cfg := multiuserComponentConfig(t)
	ingress, err := setupMultiuserIngress(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = ingress.Control.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfg.ControlDatabase)
	if err != nil {
		t.Fatal(err)
	}
	report := inspectDoctorConfig(cfg)
	if report.Multiuser == nil || report.Multiuser.Schema != 2 {
		t.Fatal("authority diagnostics missing")
	}
	after, err := os.ReadFile(cfg.ControlDatabase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("doctor mutated authority")
	}
	// TLS test fixture intentionally has no cert files; inspect the authority
	// check independently of unrelated offline TLS validation.
	found := false
	for _, check := range report.Checks {
		if check.Name == "multiuser_authority" {
			found = true
			if check.Status != "pass" {
				t.Fatal(check)
			}
		}
	}
	if !found {
		t.Fatal("authority check missing")
	}
	if err = os.Remove(cfg.ControlDatabase); err != nil {
		t.Fatal(err)
	}
	report = inspectDoctorConfig(cfg)
	for _, check := range report.Checks {
		if check.Name == "multiuser_authority" && check.Status != "fail" {
			t.Fatal("lost authority not reported")
		}
	}
	if _, err = os.Stat(cfg.ControlDatabase); !os.IsNotExist(err) {
		t.Fatal("doctor recreated missing authority")
	}
}
