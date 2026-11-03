package mailbox

import (
	"errors"
	"os"
)

// ErrWriterActive means another upgraded process already owns this authority.
// Advisory locks do not stop old binaries or administrators bypassing Store.
var ErrWriterActive = errors.New("control database already has an active writer")

func sameControlFile(path string, opened os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return errors.New("control database path changed while opening")
	}
	return nil
}

// ErrLinkedAuthority avoids different sidecar paths referring to one database.
var ErrLinkedAuthority = errors.New("control database and writer fence must have exactly one hard link")

func openWriterFence(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("control writer fence must be regular, not a symlink")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) { _ = file.Close(); return nil, err }
	info, err := file.Stat()
	if err != nil {
		return fail(err)
	}
	if !info.Mode().IsRegular() {
		return fail(errors.New("control writer fence must be regular"))
	}
	if err = singleLinkControlFile(file); err != nil {
		return fail(err)
	}
	if err = lockControlWriter(file); err != nil {
		return fail(err)
	}
	if err = sameControlFile(path, info); err != nil {
		return fail(err)
	}
	if err = file.Chmod(0600); err != nil {
		return fail(err)
	}
	// Never unlink this inode, including on Close or failed initialization.
	return file, nil
}
