//go:build !goexperiment.jsonv2

package codec

import json "encoding/json"

type RawValue = json.RawMessage

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshalJSON(in []byte, out any) error {
	return json.Unmarshal(in, out)
}

func marshalParameters(v any) (*RawValue, error) {
	if v == nil {
		return nil, nil
	}
	switch raw := v.(type) {
	case *RawValue:
		return raw, nil
	case RawValue:
		copyRaw := raw
		return &copyRaw, nil
	}
	b, err := marshalJSON(v)
	if err != nil {
		return nil, err
	}
	raw := RawValue(b)
	return &raw, nil
}

func decodeParameters(raw *RawValue, out any) error {
	if raw == nil || out == nil {
		return nil
	}
	return unmarshalJSON(*raw, out)
}
