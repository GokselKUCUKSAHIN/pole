package pole

import (
	"encoding/json"
	"fmt"
	"os"
	"pole/internal"
	"reflect"
	"time"
)

// must
// json x
// yaml

// maybe
// toml

// idk
// xml
// properties

// TAG BIND

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

// JSON

type JSONFileReader[T any] struct {
	filePath      string
	current       *T
	activeWatcher *internal.FileWatcher
}

func NewJSONFileReader[T any]() GenericFileReader[T] {
	return &JSONFileReader[T]{}
}

func (reader *JSONFileReader[T]) read(filePath string) (*T, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var conf T
	err = json.Unmarshal(data, &conf)
	return &conf, err
}

func (reader *JSONFileReader[T]) Read(filePath string) (*T, error) {
	if reader.activeWatcher != nil {
		reader.activeWatcher.Stop()
	}

	conf, err := reader.read(filePath)
	if err != nil {
		return nil, err
	}
	reader.filePath = filePath
	reader.current = conf

	watcher := internal.NewFileWatcher(filePath, 100*time.Millisecond, func() {
		// on file change
		fmt.Printf("[DEBUG] %s file changed.\n", filePath)

		newConf, err := reader.read(filePath)
		if err != nil {
			fmt.Printf("[ERROR] %s file error. could not read. reason: %s\n", filePath, err.Error())
			return
		}
		oldConf := reader.current
		reader.current = newConf

		triggerOnChange(oldConf, newConf)
	})
	if err = watcher.Start(); err != nil {
		fmt.Println("[ERROR] failed to start config file watcher")
	} else {
		reader.activeWatcher = watcher
	}

	return conf, nil
}
