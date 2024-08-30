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

//go:generate stringer -type Level -trimprefix Level
type Level uint8

const (
	LevelNONE Level = iota
	LevelFATAL
	LevelERROR
	LevelWARN
	LevelINFO
	LevelDEBUG
	LevelFINE
)

var curLvl Level = LevelINFO
var output *os.File

func Init() {
	os.Mkdir("logs", 0700)
	stat, _ := os.Stat("logs/latest.log")
	if stat != nil && stat.Size() > 0 {
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
	r, w, _ := os.Pipe()
	w2 := io.MultiWriter(os.Stderr, f)
	go io.Copy(w2, r)
	os.Stderr = w
	os.Stdout = w
}

func Cleanup() {
	os.Stderr.Close()
	output.Close()
}

func logOut(level Level, msg string) {
	if level <= curLvl {
		if level != curLvl {
			fmt.Fprint(os.Stderr, level.String(), ": ")
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
	logOut(LevelFATAL, msg)
}

func Error(msg error) {
	Errors(msg.Error())
}

func Errors(msg string) {
	logOut(LevelERROR, msg)
}

func Warn(msg string) {
	logOut(LevelWARN, msg)
}

func Info(msg string) {
	logOut(LevelINFO, msg)
}

func Debug(msg string) {
	logOut(LevelDEBUG, msg)
}

func Fine(msg string) {
	logOut(LevelFINE, msg)
}
