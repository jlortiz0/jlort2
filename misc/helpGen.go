package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func checkFatal(e error) {
	if e != nil {
		panic(e)
	}
}

// Output file format:
// asciiz name
// byte flags
// asciiz header
// byte aliases
// for each alias:
//     asciiz alias
// asciiz doc
type CmdDoc struct {
	Flags   byte // 1 GuildOnly, 2 Hidden, 4 NSFW
	Header  string
	Aliases []string
	Doc     string
}

var docs map[string]*CmdDoc = make(map[string]*CmdDoc, 64)

func loadData(file string) {
	f, err := os.Open(file)
	checkFatal(err)
	defer f.Close()
	rd := bufio.NewScanner(f)
Outer:
	for rd.Scan() {
		b := rd.Bytes()
		if len(b) < 6 || b[0] != '/' || b[1] != '/' || b[2] != ' ' || b[3] != '~' || b[4] != '!' {
			continue
		}
		doc := new(CmdDoc)
		doc.Header = string(b[3:])
		name := string(b[5:])
		spLoc := strings.IndexByte(name, ' ')
		if spLoc != -1 {
			name = name[:spLoc]
		}
		docs[name] = doc
		if !rd.Scan() {
			break
		}
		var txt string
		for rd.Bytes()[3] == '@' {
			txt = string(rd.Bytes()[4:])
			if txt == "GuildOnly" {
				doc.Flags |= 1
			} else if txt == "Hidden" {
				doc.Flags |= 2
			} else if txt == "NSFW" {
				doc.Flags |= 4
			} else if strings.HasPrefix(txt, "Alias") {
				doc.Aliases = append(doc.Aliases, txt[6:])
			}
			if !rd.Scan() {
				break Outer
			}
		}
		output := new(strings.Builder)
		for rd.Bytes()[0] == '/' {
			output.WriteByte('\n')
			output.Write(rd.Bytes()[3:])
			if !rd.Scan() {
				break Outer
			}
		}
		if output.Len() == 0 {
			fmt.Printf("WARN: %s has no description\n", name)
			delete(docs, name)
			continue
		}
		doc.Doc = output.String()[1:]
	}
	checkFatal(rd.Err())
}

func main() {
	cwd, err := os.Getwd()
	checkFatal(err)
	if strings.HasSuffix(cwd, "misc") {
		checkFatal(os.Chdir(".."))
	}
	loadData("voice.go")
	os.Chdir("modules")
	dirs, err := os.ReadDir(".")
	checkFatal(err)
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		files, err := os.ReadDir(dir.Name())
		checkFatal(err)
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".go") {
				continue
			}
			loadData(fmt.Sprintf("%s%s%s", dir.Name(), string(os.PathSeparator), file.Name()))
		}
	}
	f, err := os.OpenFile("commands"+string(os.PathSeparator)+"help.db", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	checkFatal(err)
	defer f.Close()
	wr := bufio.NewWriter(f)
	defer wr.Flush()
	for k, v := range docs {
		wr.WriteString(k)
		wr.WriteByte(0)
		wr.WriteByte(v.Flags)
		wr.WriteString(v.Header)
		wr.WriteByte(0)
		wr.WriteByte(byte(len(v.Aliases)))
		for _, a := range v.Aliases {
			wr.WriteString(a)
			wr.WriteByte(0)
		}
		wr.WriteString(v.Doc)
		wr.WriteByte(0)
	}
}
