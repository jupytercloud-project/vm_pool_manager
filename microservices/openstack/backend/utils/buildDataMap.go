package utils

import (
	"PoolManagerVM/backend/models"
	"encoding/json"
	"fmt"
)

// BuildDataMap converts a flat slice of strings into a map[string]string.
//
// Parameters:
//   - kv: A slice of strings where elements appear in key-value pairs: [key1, value1, key2, value2, ...].
//
// Returns:
//   - map[string]string: A map containing the key-value pairs.
//
// Panics:
//   - If the length of kv is not even (every key must have a corresponding value).

func BuildDataMap(kv []string) map[string]string {
	if len(kv)%2 != 0 {
		panic("BuildDataMap requires an even number of arguments (clé, valeur)")
	}

	data := make(map[string]string, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key := kv[i]
		value := kv[i+1]
		data[key] = value
	}
	return data
}

// FlatstringSP flattens a Serverpool struct into a slice of strings suitable for BuildDataMap.
//
// Parameters:
//   - p: The Serverpool struct to flatten.
//
// Returns:
//   - []string: A flat slice containing keys and values of the Serverpool's fields in the order:
//     "ID", "serverpool_id", "user_id", "image_ref", "flavor_ref", "networks", "min_vm", "max_vm".
func FlatstringSP(p models.Serverpool) []string {
	var flat []string
	flat = append(flat,
		"ID", fmt.Sprint(p.ID),
		"serverpool_id", p.ServerpoolID,
		"user_id", p.UserID,
		"image_ref", p.ImageRef,
		"flavor_ref", p.FlavorRef,
		"networks", func() string {
			b, err := json.Marshal(p.Networks)
			if err != nil {
				return "[]"
			}
			return string(b)
		}(), // JSONStringSlice, converti en string
		"min_vm", fmt.Sprint(p.MinVM),
		"max_vm", fmt.Sprint(p.MaxVM))
	return flat
}
