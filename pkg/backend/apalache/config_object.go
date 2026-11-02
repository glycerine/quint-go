package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/glycerine/insdict"
)

type configObject struct {
	values *insdict.Dict[string, json.RawMessage]
}

func readConfigObject(data json.RawMessage) (configObject, error) {
	object := configObject{values: insdict.NewDict[string, json.RawMessage]()}
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return object, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return object, err
	}
	if token != json.Delim('{') {
		return object, fmt.Errorf("expected a JSON object")
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return object, err
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return object, err
		}
		object.values.Set(key.(string), value)
	}
	if _, err = decoder.Token(); err != nil {
		return object, err
	}
	if err = decoder.Decode(new(json.RawMessage)); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing JSON value")
		}
		return object, err
	}
	return object, nil
}

func (o configObject) set(key string, value any) error {
	data, err := json.Marshal(value)
	if err == nil {
		o.values.Set(key, data)
	}
	return err
}

func (o configObject) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	first := true
	for key, value := range o.values.All() {
		if !first {
			out.WriteByte(',')
		}
		first = false
		encoded, _ := json.Marshal(key)
		out.Write(encoded)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}
