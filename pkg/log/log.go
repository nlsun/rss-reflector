package log

import (
	stdlog "log"
	"os"
)

var DefaultLogOutput = os.Stdout
var DefaultLogFlags = stdlog.Lshortfile | stdlog.Ldate | stdlog.Ltime

var DefaultLogger *stdlog.Logger = stdlog.New(DefaultLogOutput, "", DefaultLogFlags)

func PrefixedLogger(name string) *stdlog.Logger {
	return stdlog.New(DefaultLogOutput, name, DefaultLogFlags)
}
