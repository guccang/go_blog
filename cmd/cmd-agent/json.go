package main

import "encoding/json"

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
