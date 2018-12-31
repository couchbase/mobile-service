package mobile_service

import "encoding/json"

type ConfigImportExport struct {
	Listener  json.RawMessage            `json:"listener"`
	General   json.RawMessage            `json:"general"`
	Databases map[string]json.RawMessage `json:"databases"`
}
