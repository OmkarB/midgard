package model

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
)

// https://gist.github.com/JonLundy/ad750704b83aebec69749f98ba48dcdc

// MarshalFloat64 implements a Float64 value
func MarshalFloat64(t float64) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, err := io.WriteString(w, strconv.FormatFloat(t, 'f', -1, 64))
		if err != nil {
			return
		}
	})
}

// UnmarshalFloat64 implements a Float64 value
func UnmarshalFloat64(v interface{}) (float64, error) {
	switch t := v.(type) {
	case string:
		return strconv.ParseFloat(t, 64)
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case json.Number:
		return t.Float64()
	case float64:
		return t, nil
	}

	return 0, fmt.Errorf("unable to unmarshal float64: %#v %T", v, v)
}
