package mailserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

func migrationInventory(ctx context.Context, root string) ([]migrationFile, error) {
	result := make([]migrationFile, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("migration refuses symbolic links")
		}
		item := migrationFile{Path: relative, Kind: "directory"}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return errors.New("migration refuses special files")
			}
			item.Kind = "file"
			item.Size = info.Size()
			item.SHA256, err = migrationHashFile(ctx, path)
			if err != nil {
				return err
			}
		}
		result = append(result, item)
		return nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, err
}
func migrationHashFile(ctx context.Context, path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("migration requires regular files")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err = migrationCopy(ctx, hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func migrationCopy(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	var written int64
	buffer := make([]byte, 32<<10)
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		n, readErr := src.Read(buffer)
		if n > 0 {
			count, writeErr := dst.Write(buffer[:n])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != n {
				return written, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}
func migrationFingerprint(files []migrationFile) (string, error) {
	encoded, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
func copyMigrationInventory(ctx context.Context, source, destination string, files []migrationFile) error {
	for _, item := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(destination, filepath.FromSlash(item.Path))
		if item.Kind == "directory" {
			if err := os.MkdirAll(path, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := copyMigrationFile(ctx, filepath.Join(source, filepath.FromSlash(item.Path)), path); err != nil {
			return err
		}
		hash, err := migrationHashFile(ctx, path)
		if err != nil {
			return err
		}
		if hash != item.SHA256 {
			return errors.New("source changed while copying")
		}
	}
	return nil
}
func copyMigrationFile(ctx context.Context, source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("migration source must be regular")
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	if _, err = migrationCopy(ctx, output, input); err != nil {
		return err
	}
	if err = output.Sync(); err != nil {
		return err
	}
	return output.Close()
}
func writeMigrationFile(path string, data []byte) error {
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	if _, err = output.Write(data); err != nil {
		return err
	}
	if err = output.Sync(); err != nil {
		return err
	}
	return output.Close()
}
func syncMigrationDirectories(root string) error {
	var directories []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, path)
		}
		return nil
	}); err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := syncDirectory(directories[i]); err != nil {
			return err
		}
	}
	return nil
}
func verifyCompletedMigration(ctx context.Context, destination string, expected MailboxMigrationResult) (MailboxMigrationResult, error) {
	var result MailboxMigrationResult
	path := filepath.Join(destination, migrationManifestName)
	info, err := os.Lstat(path)
	if err != nil {
		return result, errors.New("destination exists without a completed migration manifest")
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return result, errors.New("invalid migration manifest")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if result.Version != 1 || result.Direction != expected.Direction || result.Namespace != expected.Namespace || result.Subject != expected.Subject || result.SourceFingerprint != expected.SourceFingerprint || result.SourceControlHash != expected.SourceControlHash || (expected.MailboxID != "" && result.MailboxID != expected.MailboxID) {
		return result, errors.New("existing migration does not match this source and target identity")
	}
	actual, err := migrationInventory(ctx, destination)
	if err != nil {
		return result, err
	}
	filtered := make([]migrationFile, 0, len(actual))
	for _, item := range actual {
		if item.Path != migrationManifestName {
			filtered = append(filtered, item)
		}
	}
	if !reflect.DeepEqual(filtered, result.Files) {
		return result, errors.New("completed migration bundle contents do not match its checksums")
	}
	if result.MailboxID == "" {
		return result, fmt.Errorf("migration manifest has no mailbox identity")
	}
	return result, nil
}
