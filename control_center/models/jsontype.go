package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type JSONStringSlice []string
type JSONStringMap map[string]string

func (j *JSONStringSlice) Scan(value any) error {
	if value == nil {
		*j = JSONStringSlice{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to scan JSONStringSlice: %v", value)
	}

	if len(bytes) == 0 {
		*j = JSONStringSlice{}
		return nil
	}

	return json.Unmarshal(bytes, j)
}

func (j JSONStringSlice) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	return json.Marshal(j)
}

func ParseJSONStringSlice(raw string) JSONStringSlice {
	var result JSONStringSlice
	if raw == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func (j *JSONStringMap) Scan(value any) error {
	if value == nil {
		*j = JSONStringMap{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to scan JSONStringMap: %v", value)
	}

	if len(bytes) == 0 {
		*j = JSONStringMap{}
		return nil
	}

	return json.Unmarshal(bytes, j)
}

func (j JSONStringMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}
