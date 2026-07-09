//go:build goexperiment.jsonv2

package codec

import (
	jsonv1 "encoding/json"
	json "encoding/json/v2"
)

type RawValue = jsonv1.RawMessage

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v, jsonv1.DefaultOptionsV1())
}

func unmarshalJSON(in []byte, out any) error {
	return json.Unmarshal(in, out, jsonv1.DefaultOptionsV1())
}

func marshalParameters(v any) (*RawValue, error) {
	if v == nil {
		return nil, nil
	}
	switch raw := v.(type) {
	case *RawValue:
		if raw == nil {
			return nil, nil
		}
		copyRaw := append(RawValue(nil), (*raw)...)
		return &copyRaw, nil
	case RawValue:
		copyRaw := append(RawValue(nil), raw...)
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
