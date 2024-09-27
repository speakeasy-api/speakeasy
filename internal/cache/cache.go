package cache

import (
	"context"
	"encoding/json"
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

func NewFileCache[T any](ctx context.Context, key string, dur time.Duration) (*FileCache[T], error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cfgDir := filepath.Join(home, ".speakeasy", "cache")
	DeleteOldCache(cfgDir, key, dur)
	return &FileCache[T]{
		dur: dur,
		dir: cfgDir,
		key: key + "-" + events.GetSpeakeasyVersionFromContext(ctx) + ".tmp.json",
	}, nil
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
		if !strings.HasPrefix(file.Name(), key) {
			continue
		}
		fileInfo, err := file.Info()
		if err != nil {
			continue
		}
		if time.Since(fileInfo.ModTime()) > dur {
			os.Remove(filepath.Join(dir, file.Name()))
		}
	}
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

	filePath := filepath.Join(c.dir, c.key)
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
