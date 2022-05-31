package model

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
)

// https://gist.github.com/JonLundy/ad750704b83aebec69749f98ba48dcdc

// MarshalInt64 implements a Int64 value
func MarshalInt64(t int64) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, err := io.WriteString(w, strconv.FormatInt(t, 10))
		if err != nil {
			return
		}
	})
}

// UnmarshalInt64 implements a Int64 value
func UnmarshalInt64(v interface{}) (int64, error) {
	switch t := v.(type) {
	case string:
		return strconv.ParseInt(t, 10, 64)
	case int:
		return int64(t), nil
	case int64:
		return int64(t), nil
	case json.Number:
		i, err := t.Int64()
		return int64(i), err
	case float64:
		return int64(t), nil
	}

	return 0, fmt.Errorf("unable to unmarshal uint64: %#v %T", v, v)
}
