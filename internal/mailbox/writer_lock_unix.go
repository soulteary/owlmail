//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package mailbox

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// Lock a persistent sidecar inode, separate from SQLite locks on the database.
func lockControlWriter(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return ErrWriterActive
	}
	if err != nil {
		return fmt.Errorf("lock control writer: %w", err)
	}
	return nil
}

func singleLinkControlFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return ErrLinkedAuthority
	}
	return nil
}
