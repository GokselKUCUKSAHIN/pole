package pole

import (
	"encoding/json"
	"fmt"
	"os"
	"pole/internal"
	"reflect"
	"strings"
	"time"
)

// must
// json ✅
// yaml

// maybe
// toml

// idk
// xml
// properties

// TAG BIND

// TODO: make it panic safe
func triggerOnChange(oldObj any, newObj any) {
	oldV, newV := reflect.ValueOf(oldObj), reflect.ValueOf(newObj)
	if oldV.Kind() != reflect.Ptr || newV.Kind() != reflect.Ptr {
		panic("pointer required")
	}

	oldElem, newElem := oldV.Elem(), newV.Elem()
	t := newElem.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		methodName := field.Tag.Get("onChange")
		if methodName == "" {
			continue
		}

		oldField, newField := oldElem.Field(i), newElem.Field(i)
		if reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
			continue
		}

		method := newV.MethodByName(methodName)
		if !method.IsValid() {
			continue
		}

		methodType := method.Type()

		var args []reflect.Value

		switch methodType.NumIn() {
		case 0:
			args = []reflect.Value{}
		case 1:
			args = []reflect.Value{
				reflect.ValueOf(oldField.Interface()),
			}
		case 2:
			args = []reflect.Value{
				reflect.ValueOf(oldField.Interface()),
				reflect.ValueOf(newField.Interface()),
			}
		default:
			continue
		}
		method.Call(args)
	}
}

// INTERFACE

type GenericFileReader[T any] interface {
	Read(filePath string) (*T, error)
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
	if strings.HasSuffix(filePath, ".json") {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		var conf T
		err = json.Unmarshal(data, &conf)
		return &conf, err
	}
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		// TODO: implement yaml
		panic("not implemented. yaml support coming soon")
	}
	return nil, fmt.Errorf("unknown file type. only json and yml (yaml) files supported")
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
