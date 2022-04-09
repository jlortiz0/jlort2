package log

import "fmt"
import "os"

type Level uint8

const (
	LevelNone Level = iota
	LevelFatal
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
	LevelFine
)

var curLvl Level = LevelInfo

func (l Level) name() string {
	switch l {
	case LevelFine:
		return "FINE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	}
	return ""
}

func logOut(level Level, msg string) {
	if level <= curLvl {
		if level != curLvl {
			fmt.Fprint(os.Stderr, level.name(), ": ")
		}
		fmt.Fprintln(os.Stderr, msg)
	}
}

func SetLevel(level Level) {
	curLvl = level
}

func GetLevel() Level {
	return curLvl
}

func Fatal(msg string) {
	logOut(LevelFatal, msg)
}

func Error(msg error) {
	Errors(msg.Error())
}

func Errors(msg string) {
	logOut(LevelError, msg)
}

func Warn(msg string) {
	logOut(LevelWarn, msg)
}

func Info(msg string) {
	logOut(LevelInfo, msg)
}

func Debug(msg string) {
	logOut(LevelDebug, msg)
}

func Fine(msg string) {
	logOut(LevelFine, msg)
}
