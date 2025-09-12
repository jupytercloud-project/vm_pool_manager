package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type JSONStringSlice []string
type JSONStringMap map[string]string

// -------- JSONStringSlice --------
func (j *JSONStringSlice) Scan(value interface{}) error {
	if value == nil {
		*j = JSONStringSlice{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSONStringSlice: %v", value)
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONStringSlice) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	return json.Marshal(j)
}

// -------- JSONStringMap --------
func (j *JSONStringMap) Scan(value interface{}) error {
	if value == nil {
		*j = JSONStringMap{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSONStringMap: %v", value)
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONStringMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}
