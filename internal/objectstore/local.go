package objectstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const keyBytes = 16

var (
	ErrTooLarge = errors.New("object is too large")
	ErrEmpty    = errors.New("object is empty")
)

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Staged interface {
	Key() string
	Size() int64
	SHA256() [sha256.Size]byte
	Sniff() []byte
	Commit() error
	Abort() error
}

type Store interface {
	Stage(context.Context, io.Reader, int64) (Staged, error)
	Open(string) (ReadSeekCloser, error)
	Delete(string) error
}
type ReconcileResult struct{ StagesDeleted, OrphansDeleted, MissingReferenced, Anomalies int }
type Reconciler interface {
	Reconcile(context.Context, map[string]struct{}, time.Time) (ReconcileResult, error)
}

type Local struct{ root string }

func OpenLocal(dataDir string) (*Local, error) {
	root := filepath.Join(dataDir, "objects")
	if info, err := os.Lstat(root); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return nil, errors.New("object root is not a directory")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect object root: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create object root: %w", err)
	}
	return &Local{root: root}, nil
}

func (store *Local) Stage(ctx context.Context, source io.Reader, limit int64) (Staged, error) {
	if err := store.checkRoot(); err != nil {
		return nil, err
	}
	key, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("generate object key: %w", err)
	}
	file, err := os.CreateTemp(store.root, ".stage-*")
	if err != nil {
		return nil, fmt.Errorf("create staged object: %w", err)
	}
	staged := &localStaged{store: store, key: key, temp: file.Name()}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = file.Close()
			_ = staged.Abort()
		}
	}()
	if err = file.Chmod(0o600); err != nil {
		return nil, fmt.Errorf("secure staged object: %w", err)
	}
	hash := sha256.New()
	sniff := &sniffWriter{}
	written, copyErr := io.Copy(io.MultiWriter(file, hash, sniff), io.LimitReader(&contextReader{ctx: ctx, reader: source}, limit+1))
	if copyErr != nil {
		return nil, fmt.Errorf("write staged object: %w", copyErr)
	}
	if written > limit {
		return nil, ErrTooLarge
	}
	if written == 0 {
		return nil, ErrEmpty
	}
	if err = file.Sync(); err != nil {
		return nil, fmt.Errorf("sync staged object: %w", err)
	}
	if err = file.Close(); err != nil {
		return nil, fmt.Errorf("close staged object: %w", err)
	}
	staged.size = written
	copy(staged.digest[:], hash.Sum(nil))
	staged.sniff = sniff.bytes
	succeeded = true
	return staged, nil
}

func (store *Local) Open(key string) (ReadSeekCloser, error) {
	if err := store.checkShard(key); err != nil {
		return nil, err
	}
	path, err := store.path(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	final, err := os.Lstat(path)
	if err != nil || final.Mode()&os.ModeSymlink != 0 || !opened.Mode().IsRegular() || !os.SameFile(opened, final) {
		file.Close()
		return nil, errors.New("object is not a regular file")
	}
	return file, nil
}

func (store *Local) Reconcile(ctx context.Context, referenced map[string]struct{}, staleBefore time.Time) (result ReconcileResult, err error) {
	entries, err := os.ReadDir(store.root)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		name := entry.Name()
		path := filepath.Join(store.root, name)
		info, infoErr := os.Lstat(path)
		if infoErr != nil {
			return result, infoErr
		}
		if strings.HasPrefix(name, ".stage-") {
			if info.Mode().IsRegular() && !info.ModTime().After(staleBefore) {
				if err := os.Remove(path); err != nil {
					return result, err
				}
				result.StagesDeleted++
			} else if !info.Mode().IsRegular() {
				result.Anomalies++
			}
			continue
		}
		if !validShard(name) || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			result.Anomalies++
			continue
		}
		children, readErr := os.ReadDir(path)
		if readErr != nil {
			return result, readErr
		}
		for _, child := range children {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			childPath := filepath.Join(path, child.Name())
			childInfo, childErr := os.Lstat(childPath)
			if childErr != nil {
				return result, childErr
			}
			key := name + child.Name()
			if !validKey(key) || !childInfo.Mode().IsRegular() {
				result.Anomalies++
				continue
			}
			if _, ok := referenced[key]; !ok && !childInfo.ModTime().After(staleBefore) {
				if err := os.Remove(childPath); err != nil {
					return result, err
				}
				result.OrphansDeleted++
			}
		}
	}
	for key := range referenced {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		path, pathErr := store.path(key)
		if pathErr != nil {
			result.MissingReferenced++
			continue
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			result.MissingReferenced++
		}
	}
	return result, nil
}

func validShard(value string) bool { return len(value) == 2 && validKey(value+strings.Repeat("0", 30)) }
func validKey(value string) bool   { _, err := (&Local{}).path(value); return err == nil }

func (store *Local) Delete(key string) error {
	if err := store.checkShard(key); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	path, err := store.path(key)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("object is not a regular file")
	}
	return os.Remove(path)
}

func (store *Local) checkRoot() error {
	info, err := os.Lstat(store.root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("object root is not a directory")
	}
	return nil
}

func (store *Local) checkShard(key string) error {
	if err := store.checkRoot(); err != nil {
		return err
	}
	path, err := store.path(key)
	if err != nil {
		return err
	}
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("object shard is not a directory")
	}
	return nil
}

func (store *Local) path(key string) (string, error) {
	if len(key) != keyBytes*2 {
		return "", errors.New("invalid object key")
	}
	decoded, err := hex.DecodeString(key)
	if err != nil || len(decoded) != keyBytes || hex.EncodeToString(decoded) != key {
		return "", errors.New("invalid object key")
	}
	return filepath.Join(store.root, key[:2], key[2:]), nil
}

type localStaged struct {
	store     *Local
	key       string
	temp      string
	size      int64
	digest    [sha256.Size]byte
	sniff     []byte
	committed bool
}

func (staged *localStaged) Key() string               { return staged.key }
func (staged *localStaged) Size() int64               { return staged.size }
func (staged *localStaged) SHA256() [sha256.Size]byte { return staged.digest }
func (staged *localStaged) Sniff() []byte             { return append([]byte(nil), staged.sniff...) }

func (staged *localStaged) Commit() error {
	if staged.committed {
		return nil
	}
	final, err := staged.store.path(staged.key)
	if err != nil {
		return err
	}
	shard := filepath.Dir(final)
	if err := os.MkdirAll(shard, 0o700); err != nil {
		return fmt.Errorf("create object shard: %w", err)
	}
	if err := staged.store.checkShard(staged.key); err != nil {
		return err
	}
	if _, err := os.Lstat(final); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return errors.New("object already exists")
		}
		return err
	}
	if err := os.Link(staged.temp, final); err != nil {
		return fmt.Errorf("finalize object: %w", err)
	}
	if err := os.Remove(staged.temp); err != nil {
		_ = os.Remove(final)
		return fmt.Errorf("remove staged object: %w", err)
	}
	staged.temp = ""
	staged.committed = true
	directory, err := os.Open(shard)
	if err != nil {
		return fmt.Errorf("open object shard: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync object shard: %w", err)
	}
	return nil
}

func (staged *localStaged) Abort() error {
	if staged.temp == "" {
		return nil
	}
	err := os.Remove(staged.temp)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	staged.temp = ""
	return err
}

func generateKey() (string, error) {
	value := make([]byte, keyBytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

type sniffWriter struct{ bytes []byte }

func (writer *sniffWriter) Write(value []byte) (int, error) {
	remaining := 512 - len(writer.bytes)
	if remaining > len(value) {
		remaining = len(value)
	}
	if remaining > 0 {
		writer.bytes = append(writer.bytes, value[:remaining]...)
	}
	return len(value), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(value []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(value)
}
