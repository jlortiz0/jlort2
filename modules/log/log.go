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

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

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
var output *os.File

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

func Init() {
	os.Mkdir("logs", 0700)
	stat, _ := os.Stat("logs/latest.log")
	if stat != nil {
		ts := stat.ModTime().Format("2006-01-02-")
		i := 1
		for {
			_, err := os.Stat(fmt.Sprintf("logs/%s%d.log.gz", ts, i))
			if err != nil && errors.Is(err, fs.ErrNotExist) {
				break
			}
			i++
		}
		in, err := os.Open("logs/latest.log")
		if err != nil {
			panic(err)
		}
		outFile, err := os.OpenFile(fmt.Sprintf("logs/%s%d.log.gz", ts, i), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		out := gzip.NewWriter(outFile)
		out.ModTime = stat.ModTime()
		out.Name = fmt.Sprintf("%s%d.log", ts, i)
		_, err = io.Copy(out, in)
		in.Close()
		if err != nil {
			panic(err)
		}
		err = out.Close()
		if err != nil {
			panic(err)
		}
	}
	f, err := os.OpenFile("logs/latest.log", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	output = f
}

func Cleanup() {
	output.Close()
}

func logOut(level Level, msg string) {
	if level <= curLvl {
		if level != curLvl {
			fmt.Fprint(os.Stderr, level.name(), ": ")
			fmt.Fprint(output, level.name(), ": ")
		}
		fmt.Fprintln(os.Stderr, msg)
		fmt.Fprintln(output, msg)
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
