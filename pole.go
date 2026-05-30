package pole

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"pole/internal"
	"reflect"
	"strings"
	"time"
)

// must
// json ✅
// yaml ✅

// maybe
// toml ✅

// idk
// xml
// properties

// TAG BIND

func triggerOnChange(oldObj, newObj any) {
	defer func() { recover() }()

	oldV, newV := reflect.ValueOf(oldObj), reflect.ValueOf(newObj)

	if oldV.Kind() != reflect.Ptr ||
		newV.Kind() != reflect.Ptr ||
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

// INTERFACE

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

	return nil, fmt.Errorf(
		"unknown file type %q, supported file types are: json, yaml, toml",
		ext,
	)
}

type FileReader[T any] struct {
	filePath      string
	current       *T
	activeWatcher *internal.FileWatcher
}

func Read[T any](filePath string) (*T, error) {
	file, err := (&FileReader[T]{}).Read(filePath)
	return file, err
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

func (reader *FileReader[T]) Read(filePath string) (*T, error) {
	if reader.activeWatcher != nil {
		reader.activeWatcher.Stop()
	}

	file, err := reader.genericReader(filePath)
	if err != nil {
		return nil, err
	}
	reader.filePath = filePath
	reader.current = file

	watcher := internal.NewFileWatcher(filePath, 100*time.Millisecond, func() {
		// on file change
		fmt.Printf("[DEBUG] %s file changed.\n", filePath)

		newFile, err := reader.genericReader(filePath)
		if err != nil {
			fmt.Printf("[ERROR] %s file error. could not genericReader. reason: %s\n", filePath, err.Error())
			return
		}
		oldFile := reader.current
		reader.current = newFile

		triggerOnChange(oldFile, newFile)
	})
	if err = watcher.Start(); err != nil {
		fmt.Println("[ERROR] failed to start config file watcher")
	} else {
		reader.activeWatcher = watcher
	}

	return file, nil
}
