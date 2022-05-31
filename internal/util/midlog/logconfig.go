package midlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

type LogConfig struct {
	Level Level `json:"level" split_words:"true"`
}

type Level zerolog.Level

func (l *Level) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "debug":
		*l = Level(zerolog.DebugLevel)
	case "info":
		*l = Level(zerolog.InfoLevel)
	case "warning":
		*l = Level(zerolog.WarnLevel)
	default:
		return errors.New(fmt.Sprintf(
			"Bad logging level: %s .Acceptable valus: debug, info, warning", s))
	}
	return nil
}

func (l Level) MarshalJSON() ([]byte, error) {
	var s string
	switch zerolog.Level(l) {
	case zerolog.DebugLevel:
		s = "debug"
	case zerolog.InfoLevel:
		s = "info"
	case zerolog.WarnLevel:
		s = "warning"
	default:
		return nil, errors.New(fmt.Sprintf("Bad logging level: %d", l))
	}

	return json.Marshal(s)
}
