package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ErrCacheMiss = errors.Error("cache miss")
)

type FileCache[T any] struct {
	dur            time.Duration
	mutex          sync.Mutex
	dir            string
	value          *T
	valueExpiresAt *time.Time
	key            string
}

type CacheSettings struct {
	Key               string
	Namespace         string
	ClearOnNewVersion bool
	Duration          time.Duration
}

func NewFileCache[T any](ctx context.Context, settings CacheSettings) (*FileCache[T], error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cfgDir := filepath.Join(home, ".speakeasy", "cache")
	DeleteOldCache(cfgDir, settings.Namespace, settings.Duration)
	builder := strings.Builder{}
	builder.WriteString(settings.Namespace)
	builder.WriteString(".")
	builder.WriteString(encode(settings.Key))
	if settings.ClearOnNewVersion {
		builder.WriteString(".")
		builder.WriteString(events.GetSpeakeasyVersionFromContext(ctx))
	}
	builder.WriteString(".tmp.json")

	return &FileCache[T]{
		dur: settings.Duration,
		dir: cfgDir,
		key: builder.String(),
	}, nil
}

func encode(key string) string {
	// hash it, trim it: we want this to be around 8 chars long
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	return hash[:8]
}

func DeleteOldCache(dir string, key string, dur time.Duration) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileInfo, err := file.Info()
		if err != nil {
			continue
		}
		if !strings.HasPrefix(file.Name(), key) {
			// special case: we never expect cache items to live more than 1 week
			if time.Since(fileInfo.ModTime()) > time.Hour*24*7 {
				os.Remove(filepath.Join(dir, file.Name()))
			}
			continue
		}
		if time.Since(fileInfo.ModTime()) > dur {
			os.Remove(filepath.Join(dir, file.Name()))
		}
	}
}

func (c *FileCache[T]) filePath() string {
	return filepath.Join(c.dir, c.key)
}

func (c *FileCache[T]) Get() (*T, error) {
	if c == nil {
		return nil, ErrCacheMiss
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.value != nil && time.Now().Before(*c.valueExpiresAt) {
		return c.value, nil
	}

	filePath := c.filePath()
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, ErrCacheMiss
	}
	if fileInfo.ModTime().Add(c.dur).Before(time.Now()) {
		_ = os.Remove(filePath)
		return nil, ErrCacheMiss
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		_ = os.Remove(filePath)
		return nil, ErrCacheMiss
	}

	value := new(T)
	if err := json.Unmarshal(fileBytes, value); err != nil {
		_ = os.Remove(filePath)
		return nil, ErrCacheMiss
	}

	return value, nil
}

func (c *FileCache[T]) Store(value *T) error {
	if c == nil {
		return nil
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Marshal the value into JSON
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Ensure the directory exists
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}

	// Write the data to a file
	filePath := filepath.Join(c.dir, c.key)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return err
	}

	// Update the value and valueExpiresAt fields
	c.value = value
	expiresAt := time.Now().Add(c.dur)
	c.valueExpiresAt = &expiresAt

	return nil
}

func (c *FileCache[T]) Delete() error {
	if c == nil {
		return nil
	}
	return os.Remove(c.filePath())
}
