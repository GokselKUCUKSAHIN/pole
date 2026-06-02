package pole

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/GokselKUCUKSAHIN/pole/internal"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

func triggerOnChange(oldObj, newObj any) {
	defer func() { recover() }()

	oldV, newV := reflect.ValueOf(oldObj), reflect.ValueOf(newObj)

	if oldV.Kind() != reflect.Pointer ||
		newV.Kind() != reflect.Pointer ||
		oldV.IsNil() ||
		newV.IsNil() {
		return
	}

	oldElem, newElem := oldV.Elem(), newV.Elem()

	if oldElem.Kind() != reflect.Struct ||
		oldElem.Type() != newElem.Type() {
		return
	}

	t := newElem.Type()

	for i := 0; i < t.NumField(); i++ {
		func() {
			defer func() { recover() }()

			methodName := t.Field(i).Tag.Get("onChange")
			if methodName == "" {
				return
			}

			oldField, newField := oldElem.Field(i).Interface(), newElem.Field(i).Interface()
			if reflect.DeepEqual(oldField, newField) {
				return
			}

			method := newV.MethodByName(methodName)
			if !method.IsValid() {
				return
			}

			switch method.Type().NumIn() {
			case 0:
				method.Call(nil)
			case 1:
				method.Call([]reflect.Value{
					reflect.ValueOf(oldField),
				})
			case 2:
				method.Call([]reflect.Value{
					reflect.ValueOf(oldField),
					reflect.ValueOf(newField),
				})
			}
		}()
	}
}

type GenericFileReader[T any] interface {
	Read(filePath string) (*T, error)
}

type UnmarshalFunc = func(data []byte, v any) error

var unmarshalFuncs = map[string]UnmarshalFunc{
	".json": json.Unmarshal,
	".yaml": yaml.Unmarshal,
	".yml":  yaml.Unmarshal,
	".toml": toml.Unmarshal,
}

func resolveUnmarshalFunc(filePath string) (UnmarshalFunc, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if fn, ok := unmarshalFuncs[ext]; ok {
		return fn, nil
	}
	return nil, fmt.Errorf("unknown file type %q, supported file types are: json, yaml, toml", ext)
}

type FileReader[T any] struct {
	filePath      string
	current       *T
	mu            sync.RWMutex
	checkInterval time.Duration
	activeWatcher *internal.FileWatcher
}

func (reader *FileReader[T]) Current() *T {
	reader.mu.RLock()
	defer reader.mu.RUnlock()
	return reader.current
}

func Read[T any](filePath string, opts ...func(*FileReader[T])) (*FileReader[T], error) {
	reader := &FileReader[T]{
		checkInterval: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(reader)
	}
	if err := reader.load(filePath); err != nil {
		return nil, err
	}
	return reader, nil
}

func WithCheckInterval[T any](d time.Duration) func(*FileReader[T]) {
	return func(r *FileReader[T]) {
		r.checkInterval = d
	}
}

func (reader *FileReader[T]) genericReader(filePath string) (*T, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	unmarshal, err := resolveUnmarshalFunc(filePath)
	if err != nil {
		return nil, err
	}
	var conf T
	err = unmarshal(data, &conf)
	return &conf, err
}

func (reader *FileReader[T]) load(filePath string) error {
	if reader.activeWatcher != nil {
		reader.activeWatcher.Stop()
	}

	file, err := reader.genericReader(filePath)
	if err != nil {
		return err
	}
	reader.filePath = filePath
	reader.mu.Lock()
	reader.current = file
	reader.mu.Unlock()

	watcher := internal.NewFileWatcher(filePath, reader.checkInterval, func() {
		newFile, err := reader.genericReader(filePath)
		if err != nil {
			logErrorf("%s file error. file could not read. reason: %s", filePath, err.Error())
			return
		}
		reader.mu.Lock()
		oldFile := reader.current
		reader.current = newFile
		reader.mu.Unlock()

		triggerOnChange(oldFile, newFile)
	})
	if err = watcher.Start(); err != nil {
		logErrorf("failed to start config file watcher")
	} else {
		reader.activeWatcher = watcher
	}

	return nil
}

func logErrorf(errorMessage string, args ...any) {
	msg := fmt.Sprintf(errorMessage, args...)
	ts := time.Now().UTC().Format(time.RFC3339)
	b, err := json.Marshal(map[string]any{"level": "ERROR", "msg": msg, "time": ts})
	if err != nil {
		fmt.Printf(`{"level":"ERROR","msg":"%s","time":"%s"}`, msg, ts)
		return
	}
	fmt.Println(string(b))
}
