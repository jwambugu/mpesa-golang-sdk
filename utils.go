package mpesa

import (
	"encoding/json"
	"io"
	"net/http"
)

// toBytes decodes the provided value to bytes
func toBytes(in interface{}) (out []byte, err error) {
	switch v := in.(type) {
	case string:
		return []byte(v), nil
	case *http.Request:
		out, err = io.ReadAll(v.Body)
		if err != nil {
			return nil, err
		}
		return out, nil
	default:
		if out, err = json.Marshal(v); err != nil {
			return nil, err
		}
		return out, nil
	}
}
