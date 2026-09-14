package objectstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

type Local struct {
	root      string
	confined  *os.Root
	closeOnce sync.Once
	closeErr  error
}

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
	confined, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open object root: %w", err)
	}
	return &Local{root: root, confined: confined}, nil
}
func (store *Local) Close() error {
	store.closeOnce.Do(func() { store.closeErr = store.confined.Close() })
	return store.closeErr
}

func (store *Local) Stage(ctx context.Context, source io.Reader, limit int64) (Staged, error) {
	key, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("generate object key: %w", err)
	}
	var temp string
	var file *os.File
	for range 8 {
		temp = ".stage-" + key + "-" + randomSuffix()
		file, err = store.confined.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		break
	}
	if err != nil {
		return nil, fmt.Errorf("create staged object: %w", err)
	}
	staged := &localStaged{store: store, key: key, temp: temp}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = file.Close()
			_ = staged.Abort()
		}
	}()
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
	rel, err := relative(key)
	if err != nil {
		return nil, err
	}
	file, err := store.confined.Open(rel)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	final, err := store.confined.Lstat(rel)
	if err != nil || final.Mode()&os.ModeSymlink != 0 || !opened.Mode().IsRegular() || !os.SameFile(opened, final) {
		file.Close()
		return nil, errors.New("object is not a regular file")
	}
	return file, nil
}

func (store *Local) Delete(key string) error {
	rel, err := relative(key)
	if err != nil {
		return err
	}
	info, err := store.confined.Lstat(rel)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("object is not a regular file")
	}
	return store.confined.Remove(rel)
}

func (store *Local) Reconcile(ctx context.Context, referenced map[string]struct{}, staleBefore time.Time) (result ReconcileResult, err error) {
	entries, err := fs.ReadDir(store.confined.FS(), ".")
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		name := entry.Name()
		info, infoErr := store.confined.Lstat(name)
		if infoErr != nil {
			return result, infoErr
		}
		if strings.HasPrefix(name, ".stage-") {
			if info.Mode().IsRegular() && !info.ModTime().After(staleBefore) {
				if err := store.confined.Remove(name); err != nil {
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
		children, readErr := fs.ReadDir(store.confined.FS(), name)
		if readErr != nil {
			return result, readErr
		}
		for _, child := range children {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			rel := name + "/" + child.Name()
			childInfo, childErr := store.confined.Lstat(rel)
			if childErr != nil {
				return result, childErr
			}
			key := name + child.Name()
			if !validKey(key) || !childInfo.Mode().IsRegular() {
				result.Anomalies++
				continue
			}
			if _, ok := referenced[key]; !ok && !childInfo.ModTime().After(staleBefore) {
				if err := store.confined.Remove(rel); err != nil {
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
		rel, relErr := relative(key)
		if relErr != nil {
			result.MissingReferenced++
			continue
		}
		info, statErr := store.confined.Lstat(rel)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			result.MissingReferenced++
		}
	}
	return result, nil
}

func (store *Local) path(key string) (string, error) {
	rel, err := relative(key)
	if err != nil {
		return "", err
	}
	return filepath.Join(store.root, filepath.FromSlash(rel)), nil
}
func relative(key string) (string, error) {
	if !validKey(key) {
		return "", errors.New("invalid object key")
	}
	return key[:2] + "/" + key[2:], nil
}
func validShard(value string) bool { return len(value) == 2 && validKey(value+strings.Repeat("0", 30)) }
func validKey(value string) bool {
	decoded, err := hex.DecodeString(value)
	return len(value) == keyBytes*2 && err == nil && len(decoded) == keyBytes && hex.EncodeToString(decoded) == value
}

type localStaged struct {
	store     *Local
	key, temp string
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
	rel, err := relative(staged.key)
	if err != nil {
		return err
	}
	shard := staged.key[:2]
	if err := staged.store.confined.MkdirAll(shard, 0o700); err != nil {
		return fmt.Errorf("create object shard: %w", err)
	}
	if info, err := staged.store.confined.Lstat(rel); !errors.Is(err, os.ErrNotExist) {
		if err == nil && info != nil {
			return errors.New("object already exists")
		}
		return err
	}
	if err := staged.store.confined.Link(staged.temp, rel); err != nil {
		return fmt.Errorf("finalize object: %w", err)
	}
	if err := staged.store.confined.Remove(staged.temp); err != nil {
		_ = staged.store.confined.Remove(rel)
		return fmt.Errorf("remove staged object: %w", err)
	}
	staged.temp = ""
	staged.committed = true
	directory, err := staged.store.confined.Open(shard)
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
	err := staged.store.confined.Remove(staged.temp)
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
func randomSuffix() string {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(value)
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
