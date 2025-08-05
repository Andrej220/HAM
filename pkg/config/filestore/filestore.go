package filestore

import (
	"os"
	"gopkg.in/yaml.v3"
	"github.com/fsnotify/fsnotify"
	"log"
	"fmt"
	"github.com/andrej220/HAM/pkg/config/configstore"
	//"github.com/andrej220/HAM/pkg/lg"
)

var _ configstore.ConfigStore = (*FileStore)(nil)

type FileStore struct {
	Path string
}

func New(path string) *FileStore {
	return &FileStore{Path: path}
}

func WriteSecureFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func (f *FileStore) Load(out any)  error {
	if out == nil {
		return fmt.Errorf("Load: output parameter must not be nil")
	}

	bytes, err := os.ReadFile(f.Path)
	if err != nil {
		return fmt.Errorf("Load: failed to read file %s: %w", f.Path, err)
	}

	if len(bytes) == 0 {
		return fmt.Errorf("Load: config file %s is empty", f.Path)
	}

	if err := yaml.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("Load: failed to parse YAML in %s: %w", f.Path, err)
	}

	return  nil
}

func (f *FileStore) Save(in any) error {
	if in == nil {
		return fmt.Errorf("Save: input parameter must not be nil")
	}

	bytes, err := yaml.Marshal(in)
	if err != nil {
		return fmt.Errorf("Save: failed to marshal YAML: %w", err)
	}

	// Write to temp file first
	tmpPath := f.Path + ".tmp"
	err = os.WriteFile(tmpPath, bytes, 0600)
	if err != nil {
		return fmt.Errorf("Save: failed to write temp file %s: %w", tmpPath, err)
	}

	// Atomic rename
	err = os.Rename(tmpPath, f.Path)
	if err != nil {
		return fmt.Errorf("Save: failed to replace %s with %s: %w", f.Path, tmpPath, err)
	}

	return nil
}

func (f *FileStore) Watch(onChange func()) error {
	if onChange == nil {
        return fmt.Errorf("onChange callback cannot be nil")
    }

	watcher, err := fsnotify.NewWatcher()
    if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
    }
	
	err = watcher.Add(f.Path)
	if err != nil {
        return fmt.Errorf("failed to watch file %s: %w", f.Path, err)
    }

    go func() {
		defer watcher.Close()
        for {
            select {
            case event, ok := <-watcher.Events:
				if !ok{
					return
				}
                if event.Op&fsnotify.Write != 0 {
                    onChange()
                }
            case err,  ok := <-watcher.Errors:
				if !ok {
                    return
                }
                log.Printf("Watcher error on %s: %v", f.Path, err)
            }
        }
    }()

	return nil
}
