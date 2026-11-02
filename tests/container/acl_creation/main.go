//go:build owlmail_acl_creation_fixture

// Test-only syscall creation-mode probe; excluded from normal builds.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func run() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: acl-creation /state/parent prefix")
	}
	parent, err := filepath.EvalSymlinks(os.Args[1])
	if err != nil {
		return err
	}
	if parent != "/state" && !strings.HasPrefix(parent, "/state/") {
		return fmt.Errorf("parent must stay inside test state")
	}
	prefix := os.Args[2]
	if prefix == "" || filepath.Base(prefix) != prefix || strings.Contains(prefix, "..") {
		return fmt.Errorf("invalid prefix")
	}
	// Both calls supply restrictive mode at the actual creation syscall; there
	// is no chmod/install step that could simulate the expected effective mask.
	syscall.Umask(0077)
	file, err := os.OpenFile(filepath.Join(parent, prefix+"-narrow"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	private := filepath.Join(parent, prefix+"-private-directory")
	if err = os.Mkdir(private, 0700); err != nil {
		return err
	}
	child, err := os.OpenFile(filepath.Join(private, "child"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	return child.Close()
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
