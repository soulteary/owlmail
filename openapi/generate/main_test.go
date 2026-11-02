package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedMultiuserDocumentsAreCurrent(t *testing.T) {
	input, err := os.ReadFile("../openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]byte{}
	for _, name := range []string{"multiuser.json", "multiuser.yaml"} {
		value, err := os.ReadFile("../" + name)
		if err != nil {
			t.Fatal(err)
		}
		expected[name] = value
	}
	directory := t.TempDir()
	if err = os.WriteFile(filepath.Join(directory, "openapi.json"), input, 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	if err = generate(); err != nil {
		t.Fatal(err)
	}
	for name, want := range expected {
		got, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run go generate ./openapi", name)
		}
	}
	unchanged, err := os.ReadFile("openapi.json")
	if err != nil || !bytes.Equal(input, unchanged) {
		t.Fatal("generator changed standalone input", err)
	}
}
