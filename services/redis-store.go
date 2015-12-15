package services

import (
	"fmt"
	"reflect"
	"strconv"
)

func iterateStructFields(structVal reflect.Value, do func(name string, fieldValue reflect.Value)) {
	typ := structVal.Type()
	for i := 0; i < typ.NumField(); i++ {
		fieldType := typ.Field(i)
		tag := fieldType.Tag.Get("redis")
		if tag != "-" {
			field := structVal.Field(i)
			if fieldType.Anonymous && field.Kind() == reflect.Struct {
				// Recursively iterate anonymuos struct fields.
				iterateStructFields(field, do)
			} else if fieldType.PkgPath == "" && redisCanParse(fieldType.Type.Kind()) {
				// Exported, non-composite struct field
				name := fieldType.Name
				if tag != "" {
					name = tag
				}
				do(name, field)
			}
		}
	}
}

// ============================ Saving ============================

func AsMap(obj interface{}) map[string]interface{} {
	val := reflect.ValueOf(obj)
	if val.Type().Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Type().Kind() != reflect.Struct {
		return nil
	}
	values := make(map[string]interface{})
	iterateStructFields(val, func(name string, fieldValue reflect.Value) {
		values[name] = fieldValue.Interface()
	})
	return values
}

func (r *redis) StoreStruct(key string, obj interface{}) error {
	values := AsMap(obj)
	if values == nil {
		return fmt.Errorf("Non-struct value: %T %v", obj, obj)
	}
	return r.Cmd("hmset", key, values).Err()
}

func redisCanParse(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return true
	default:
		return false
	}
}

// ============================ Loading ============================

func parseAndSet(target reflect.Value, val string) error {
	if !target.CanSet() {
		return fmt.Errorf("Cannot set %v to %v", target, val)
	}
	switch kind := target.Type().Kind(); kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			target.SetInt(intVal)
		}
		return err
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		intVal, err := strconv.ParseUint(val, 10, 64)
		if err == nil {
			target.SetUint(intVal)
		}
		return err
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(val, 64)
		if err == nil {
			target.SetFloat(floatVal)
		}
		return err
	case reflect.String:
		target.SetString(val)
		return nil
	}
	return fmt.Errorf("Field %v has type %v, cannot set to %v", target, target.Type(), val)
}

func FillFromMap(values map[string]string, obj interface{}) error {
	val := reflect.ValueOf(obj)
	if val.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("Not a pointer: %v", obj)
	}
	val = val.Elem()
	if val.Type().Kind() != reflect.Struct {
		return fmt.Errorf("Not a struct pointer: %v", obj)
	}

	fieldNames := make(map[string]reflect.Value)
	iterateStructFields(val, func(name string, fieldValue reflect.Value) {
		fieldNames[name] = fieldValue
	})

	for key, value := range values {
		if field, ok := fieldNames[key]; ok {
			_ = parseAndSet(field, value) // Ignore errors for individual fields
		}
	}
	return nil
}

func (r *redis) LoadStruct(key string, obj interface{}) error {
	values, err := r.Cmd("hgetall", key).Map()
	if err != nil {
		return err
	}
	FillFromMap(values, obj)
	return nil
}

// ============================ Support ============================

type Storable interface {
	Key() string
	Client() Redis
}

type StoredObject struct {
	S Storable
}

func (stored *StoredObject) Exists() (bool, error) {
	return stored.S.Client().Cmd("exists", stored.S.Key()).Bool()
}

func (stored *StoredObject) Save() error {
	return stored.S.Client().StoreStruct(stored.S.Key(), stored.S)
}

func (stored *StoredObject) Load() error {
	return stored.S.Client().LoadStruct(stored.S.Key(), stored.S)
}

func (stored *StoredObject) LoadExisting() (bool, error) {
	if exists, err := stored.Exists(); !exists {
		return false, err
	}
	err := stored.Load()
	return err == nil, err
}
