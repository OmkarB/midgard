package midlog

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var GlobalLogger Logger
var exitFunction func()
var subloggers = map[string]*Logger{}

func init() {
	SetGlobalOutput(os.Stdout, false)
}

// log.Fatal calls os.Exit which prevents test cleanups (printing of logs)
// Tests can set t.FailNow to be called instead of os.Exit
func SetExitFunctionForTest(f func()) {
	exitFunction = f
}

func SetFromConfig(config LogConfig) {
	zerolog.SetGlobalLevel(zerolog.Level(config.Level))
}

// Not thread safe, call it durring global initialization or test initialization
func SetGlobalOutput(w io.Writer, noColor bool) {
	log.Logger = log.Output(
		zerolog.ConsoleWriter{
			Out:        w,
			TimeFormat: "2006-01-02 15:04:05",
			PartsOrder: []string{"level", "time", "caller", "message"},
			NoColor:    noColor,
		},
	)
	GlobalLogger.zlog = log.Logger
	refreshSubloggers()
}

func LoggerForModule(module string) *Logger {
	l := newSublogger(module)
	subloggers[module] = &l
	return &l
}

func newSublogger(module string) Logger {
	return Logger{GlobalLogger.zlog.With().Str("module", module).Logger()}
}

func refreshSubloggers() {
	for module, l := range subloggers {
		*l = newSublogger(module)
	}
}
