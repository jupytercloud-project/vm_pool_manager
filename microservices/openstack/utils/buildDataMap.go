package utils

import (
	"PoolManagerVM/backend/models"
	"encoding/json"
	"fmt"
)

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

func FlatstringSP(p models.Serverpool) []string {
	var flat []string
	if p.NetworkUuid != "" {
		p.Networks = []string{p.NetworkUuid}
	}
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
		}(),
		"min_vm", fmt.Sprint(p.MinVM),
		"max_vm", fmt.Sprint(p.MaxVM),
		"config_id", fmt.Sprint(p.ConfigID))
	return flat
}
