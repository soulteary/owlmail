package mailbox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestControlWriterProcess(t *testing.T) {
	path := os.Getenv("OWLMAIL_TEST_CONTROL_WRITER")
	if path == "" {
		return
	}
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if _, err = fmt.Fprintln(os.Stdout, "writer-ready"); err != nil {
		t.Fatal(err)
	}
	var input [1]byte
	_, _ = os.Stdin.Read(input[:])
}

func TestControlWriterLockRejectsSecondProcessAndReleasesOnCrash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	initial, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = initial.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestControlWriterProcess$")
	child.Env = append(os.Environ(), "OWLMAIL_TEST_CONTROL_WRITER="+path)
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close() }()
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		if !waited {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "writer-ready" {
		t.Fatalf("child not ready: %q %v", scanner.Text(), scanner.Err())
	}
	if second, err := Open(path, false); !errors.Is(err, ErrWriterActive) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatalf("second process admitted: %v", err)
	}
	before, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reader.Inspect(ctx); err != nil {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(after) != len(before) {
		t.Fatalf("read-only inspection created a file: %v %v", after, err)
	}
	if err = child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err = child.Wait(); err == nil {
		t.Fatal("killed process exited successfully")
	}
	waited = true
	recovered, err := Open(path, false)
	if err != nil {
		t.Fatalf("crash did not release writer lock: %v", err)
	}
	if err = recovered.Close(); err != nil {
		t.Fatal(err)
	}
	if err = recovered.Close(); err != nil {
		t.Fatal("repeated close", err)
	}
}

func TestControlWriterLockSameProcessAndAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	paths := []string{path, filepath.Join(filepath.Dir(path), ".", "control.sqlite")}
	alias := filepath.Join(filepath.Dir(path), "hardlink.sqlite")
	if err = os.Link(path, alias); err == nil {
		paths = append(paths, alias)
	} else if runtime.GOOS != "windows" {
		t.Fatal(err)
	}
	for _, candidate := range paths {
		second, err := Open(candidate, false)
		if second != nil {
			_ = second.Close()
		}
		if !errors.Is(err, ErrWriterActive) && !errors.Is(err, ErrLinkedAuthority) {
			t.Fatalf("writer alias admitted %s: %v", candidate, err)
		}
	}
	link := filepath.Join(filepath.Dir(path), "symlink.sqlite")
	if err = os.Symlink(path, link); err == nil {
		if second, err := Open(link, false); err == nil {
			_ = second.Close()
			t.Fatal("symlink writer allowed")
		}
	}
}

func TestControlWriterOpenFailureReleasesLockAndDoesNotChmodLockedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.sqlite")
	if err := os.WriteFile(path, []byte("not a SQLite database"), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		s, err := Open(path, false)
		if s != nil {
			_ = s.Close()
		}
		if err == nil || errors.Is(err, ErrWriterActive) {
			t.Fatalf("failed open leaked writer lock: %v", err)
		}
	}
	path = filepath.Join(t.TempDir(), "valid.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if runtime.GOOS == "windows" {
		return
	}
	if err = os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	if second, err := Open(path, false); !errors.Is(err, ErrWriterActive) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal("rejected writer changed permissions", info, err)
	}
}

func TestControlWriterCloseWaitsForInFlightAuthorityOperation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	// A close cannot release the inode fence while an authority callback owns mu.
	select {
	case err := <-closed:
		s.mu.Unlock()
		t.Fatal("close overtook operation", err)
	case <-time.After(20 * time.Millisecond):
	}
	if second, err := Open(path, false); !errors.Is(err, ErrWriterActive) {
		if second != nil {
			_ = second.Close()
		}
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestControlWriterFenceRejectsSymlinksAndHardlinks(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "control.sqlite")
			s, err := Open(path, true)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "operator-file")
			if err = os.WriteFile(target, []byte("preserve me"), 0644); err != nil {
				t.Fatal(err)
			}
			fence := path + ".writer-lock"
			// This is a stopped, private test fixture; never replace a live fence.
			if err = os.Remove(fence); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				err = os.Symlink(target, fence)
			} else {
				err = os.Link(target, fence)
			}
			if err != nil && runtime.GOOS == "windows" {
				t.Skip("link creation unavailable", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			if other, err := Open(path, false); err == nil {
				_ = other.Close()
				t.Fatal("linked fence accepted")
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "preserve me" {
				t.Fatal("linked target modified", err)
			}
			if runtime.GOOS != "windows" {
				info, err := os.Stat(target)
				if err != nil || info.Mode().Perm() != 0644 {
					t.Fatal("linked target chmod", err)
				}
			}
		})
	}
}

func TestControlWriterParentAliasUsesTheSameFence(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "control.sqlite")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	alias := filepath.Join(t.TempDir(), "parent")
	if err = os.Symlink(parent, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("directory symlink unavailable", err)
		}
		t.Fatal(err)
	}
	if other, err := Open(filepath.Join(alias, "control.sqlite"), false); !errors.Is(err, ErrWriterActive) {
		if other != nil {
			_ = other.Close()
		}
		t.Fatalf("parent alias bypassed fence: %v", err)
	}
}
