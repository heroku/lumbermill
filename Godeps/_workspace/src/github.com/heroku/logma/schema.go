package logma

import "reflect"

// Envelope represents a schematized malahat message
type Envelope struct {
	Type  string      `json:"_type"`
	Time  int64       `json:"_time"`
	Owner string      `json:"_owner"`
	Value interface{} `json:"_value"`
}

func typename(v interface{}) string {
	t := reflect.TypeOf(v)
	switch t.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map,
		reflect.Ptr, reflect.Slice:
		return t.Elem().Name()
	default:
		return t.Name()
	}
}
