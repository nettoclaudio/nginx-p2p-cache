package nginx

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type CacheEntry struct {
	ID           string
	Filename     string
	Modification time.Time
	RemovedAt    time.Time
}

type CacheWatcher struct {
	data    sync.Map
	added   chan string
	removed chan string
}

func (cw *CacheWatcher) Added() <-chan string {
	return cw.added
}

func (cw *CacheWatcher) Removed() <-chan string {
	return cw.removed
}

func (cw *CacheWatcher) Watch(ctx context.Context, directory string) error {
	cw.added = make(chan string)
	cw.removed = make(chan string)
	defer close(cw.added)
	defer close(cw.removed)

	if err := ctx.Err(); err != nil {
		return err
	}

	if ok, _ := IsDir(directory); !ok {
		return errors.New("path is not directory")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := cw.fullSync(watcher, directory); err != nil {
		return err
	}

	for {
		select {
		case evt, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("events channel is closed")
			}

			go cw.handleEvent(watcher, evt)

		case <-ctx.Done():
			fmt.Println("Context canceled, finishing watcher...")
			return nil
		}
	}
}

func (cw *CacheWatcher) Keys() (keys []string) {
	cw.data.Range(func(key, _ any) bool {
		keys = append(keys, key.(string))
		return true
	})

	return
}

func (cw *CacheWatcher) fullSync(watcher *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, fs.WalkDirFunc(func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return watcher.Add(path)
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		if fi.Mode().IsRegular() {
			cw.addFile(path)
			return nil
		}

		return nil
	}))
}

func (cw *CacheWatcher) handleEvent(watcher *fsnotify.Watcher, event fsnotify.Event) {
	filename := event.Name

	if event.Op.Has(fsnotify.Create) || event.Op.Has(fsnotify.Write) {
		if ok, _ := IsDir(filename); ok {
			watcher.Add(filename)
			return
		}

		cw.addFile(filename)
		return
	}

	if event.Op.Has(fsnotify.Remove) {
		if ok, _ := IsDir(filename); ok {
			watcher.Remove(filename)
			return
		}

		cw.deleteFile(filename)
		return
	}
}

func (cw *CacheWatcher) addFile(filename string) {
	ce, err := Unmarshal(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal cache entry: %s\n", err)
	}

	key := filepath.Base(filename)

	_, found := cw.data.Swap(key, ce)
	if !found {
		cw.added <- key
	}
}

func (cw *CacheWatcher) deleteFile(filename string) {
	key := filepath.Base(filename)
	cw.data.Delete(key)
	cw.removed <- key
}

func Unmarshal(filename string) (CacheEntry, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return CacheEntry{}, err
	}

	return CacheEntry{
		ID:           filepath.Base(filename),
		Filename:     filename,
		Modification: fi.ModTime(),
	}, nil
}

func IsDir(name string) (bool, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return false, err
	}

	return fi.IsDir(), nil
}
