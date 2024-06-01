package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func checkFatal(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	cwd, err := os.Getwd()
	checkFatal(err)
	if strings.HasSuffix(cwd, "dbGen") {
		checkFatal(os.Chdir(".."))
	}
	os.Chdir("persistent")
	_, err = os.Stat("../persistent.db")
	if err == nil {
		fmt.Print("[o]verwrite, [i]nsert, [a]bort? ")
		var b [16]byte
		os.Stdin.Read(b[:])
		if b[0] == 'o' {
			os.Remove("../persistent.db")
		} else if b[0] == 'a' {
			return
		} else if b[0] != 'i' {
			fmt.Println("specify one of o, i, a")
			return
		}
	}
	db, err := sql.Open("sqlite3", "../persistent.db")
	checkFatal(err)
	defer db.Close()
	fmt.Println("opened db")

	db.Exec("CREATE TABLE vachan (gid INTEGER PRIMARY KEY, cid INTEGER NOT NULL);")
	data, err := os.ReadFile("vachan")
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	} else if err == nil {
		var vaChannels map[string]string
		checkFatal(json.Unmarshal(data, &vaChannels))
		fmt.Println("read vachan")
		stmt, _ := db.Prepare("INSERT INTO vachan VALUES (?, ?);")
		for k, v := range vaChannels {
			if v == "" || k == "" {
				continue
			}
			k2, _ := strconv.ParseInt(k, 10, 64)
			v2, _ := strconv.ParseInt(v, 10, 64)
			stmt.Exec(k2, v2)
		}
		stmt.Close()
		fmt.Println("inserted vachan")
	}

	db.Exec("CREATE TABLE djRole (gid INTEGER PRIMARY KEY, rid INTEGER NOT NULL);")
	data, err = os.ReadFile("dj")
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	} else if err == nil {
		var vaChannels map[string]string
		checkFatal(json.Unmarshal(data, &vaChannels))
		fmt.Println("read dj")
		stmt, _ := db.Prepare("INSERT INTO djRole VALUES (?, ?);")
		for k, v := range vaChannels {
			if v == "" || k == "" {
				continue
			}
			k2, _ := strconv.ParseInt(k, 10, 64)
			v2, _ := strconv.ParseInt(v, 10, 64)
			stmt.Exec(k2, v2)
		}
		stmt.Close()
		fmt.Println("inserted dj")
	}

	db.Exec("CREATE TABLE quotes (gid UNSIGNED BIGINT, ind UNSIGNED INTEGER, quote VARCHAR(512) NOT NULL, PRIMARY KEY(gid, ind));")
	data, err = os.ReadFile("quotes")
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	} else if err == nil {
		var vaChannels map[string][]string
		checkFatal(json.Unmarshal(data, &vaChannels))
		fmt.Println("read quotes")
		stmt, _ := db.Prepare("INSERT INTO quotes VALUES (?, ?, ?);")
		for k, v := range vaChannels {
			if len(v) == 0 || k == "" {
				continue
			}
			k2, _ := strconv.ParseInt(k, 10, 64)
			for i, s := range v {
				stmt.Exec(k2, i+1, s)
			}
		}
		stmt.Close()
		fmt.Println("inserted quotes")
	}

	db.Exec("CREATE TABLE kekGuilds (gid INTEGER PRIMARY KEY);")
	db.Exec("CREATE TABLE kekUsers (uid INTEGER PRIMARY KEY, score INTEGER DEFAULT 0 NOT NULL);")
	db.Exec("CREATE TABLE kekMsgs (uid UNSIGNED BIGINT REFERENCES kekUsers, mid UNSIGNED BIGINT, score INTEGER DEFAULT 0 NOT NULL, PRIMARY KEY (uid, mid) ON CONFLICT REPLACE);")
	db.Exec("CREATE TRIGGER KekNewUser BEFORE INSERT ON kekMsgs FOR EACH ROW BEGIN INSERT OR IGNORE INTO kekUsers (uid) VALUES (new.uid); END;")
	data, err = os.ReadFile("kek")
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	} else if err == nil {
		var vaChannels struct {
			Guilds map[string]interface{}
			Users  map[string]map[string]int
		}
		checkFatal(json.Unmarshal(data, &vaChannels))
		fmt.Println("read kek")
		stmt, _ := db.Prepare("INSERT INTO kekGuilds VALUES (?);")
		for k := range vaChannels.Guilds {
			k2, _ := strconv.ParseInt(k, 10, 64)
			stmt.Exec(k2)
		}
		stmt.Close()
		stmt, _ = db.Prepare("INSERT INTO kekUsers VALUES (?, ?);")
		stmt2, _ := db.Prepare("INSERT INTO kekMsgs VALUES (?, ?, ?);")
		for k, v := range vaChannels.Users {
			if k == "" || (len(v) < 2 && v["locked"] == 0) {
				continue
			}
			k2, _ := strconv.ParseInt(k, 10, 64)
			stmt.Exec(k2, v["locked"])
			delete(v, "locked")
			for i, s := range v {
				if s == 0 {
					continue
				}
				i2, _ := strconv.ParseInt(i, 10, 64)
				stmt2.Exec(k2, i2, s)
			}
		}
		stmt.Close()
		stmt2.Close()
		fmt.Println("inserted kek")
	}
	db.Exec("CREATE TABLE reminders (ts TIMESTAMP NOT NULL, uid INTEGER NOT NULL, created TIMESTAMP NOT NULL, what VARCHAR(2000) NOT NULL, PRIMARY KEY (uid, created));")
	db.Exec("CREATE INDEX remindTs ON reminders (ts);")
	db.Exec("CREATE TABLE userTz (uid INTEGER PRIMARY KEY, tz VARCHAR(31) NOT NULL);")
}
