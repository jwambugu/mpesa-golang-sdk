package mpesa

import "encoding/json"

// toBytes decodes the provided value to bytes
func toBytes(in interface{}) (out []byte, err error) {
	switch v := in.(type) {
	case string:
		return []byte(v), nil
	default:
		if out, err = json.Marshal(v); err != nil {
			return nil, err
		}
		return out, nil
	}
}
