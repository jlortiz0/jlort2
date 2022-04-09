/*
Copyright (C) 2021-2022 jlortiz

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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
