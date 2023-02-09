package main

import "fmt"
import "strings"
import "strconv"
import "encoding/json"
import "os"
import "time"
import "database/sql"
import "github.com/bwmarrin/discordgo"
import _ "github.com/mattn/go-sqlite3"

func checkFatal(err error) {
    if err != nil {
        panic(err)
    }
}

func main() {
    cwd, err := os.Getwd()
    checkFatal(err)
    if strings.HasSuffix(cwd, "misc") {
        checkFatal(os.Chdir(".."))
    }
    os.Chdir("persistent")
    db, err := sql.Open("sqlite3", "../persistent.db")
    checkFatal(err)
    defer db.Close()
    fmt.Println("opened db")

    db.Exec("CREATE TABLE vachan (gid UNSIGNED BIGINT PRIMARY KEY, cid UNSIGNED BIGINT NOT NULL);")
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

    db.Exec("CREATE TABLE djRole (gid UNSIGNED BIGINT PRIMARY KEY, rid UNSIGNED BIGINT NOT NULL);")
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

    db.Exec("CREATE TABLE songAlias (gid UNSIGNED BIGINT, key VARCHAR(32), value VARCHAR(128) NOT NULL, PRIMARY KEY(gid, key));")
    data, err = os.ReadFile("song")
    if err != nil && !os.IsNotExist(err) {
        panic(err)
    } else if err == nil {
        var vaChannels map[string]map[string]string
        checkFatal(json.Unmarshal(data, &vaChannels))
        fmt.Println("read song")
        stmt, _ := db.Prepare("INSERT INTO songAlias VALUES (?, ?, ?);")
        for gid, sm := range vaChannels {
            gint, _ := strconv.ParseInt(gid, 10, 64)
            for k, v := range sm {
                if v == "" || k == "" {
                    continue
                }
                stmt.Exec(gint, k, v)
            }
        }
        stmt.Close()
        fmt.Println("inserted song")
    }

    db.Exec("CREATE TABLE brit (uid UNSIGNED BIGINT PRIMARY KEY, score TINYINT NOT NULL DEFAULT 50);")
    data, err = os.ReadFile("brit")
    if err != nil && !os.IsNotExist(err) {
        panic(err)
    } else if err == nil {
        var vaChannels map[string]int
        checkFatal(json.Unmarshal(data, &vaChannels))
        fmt.Println("read brit")
        stmt, _ := db.Prepare("INSERT INTO brit VALUES (?, ?);")
        for k, v := range vaChannels {
            if v == 50 || k == "" {
                continue
            }
            k2, _ := strconv.ParseInt(k, 10, 64)
            stmt.Exec(k2, v)
        }
        stmt.Close()
        fmt.Println("inserted brit")
    }

    db.Exec("CREATE TABLE quotes (gid UNSIGNED BIGINT, ind UNSIGNED INTEGER, quote VARCHAR(512), PRIMARY KEY(gid, ind));")
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

    db.Exec("CREATE TABLE gachaPlayer (uid UNSIGNED BIGINT PRIMARY KEY, tokens UNSIGNED INTEGER DEFAULT 0 NOT NULL, nextPull TIMESTAMP);")
    db.Exec("CREATE TABLE gachaItems (uid UNSIGNED BIGINT REFERENCES gachaPlayer, itemId UNSIGNED INTEGER, count UNSIGNED INTEGER NOT NULL, PRIMARY KEY(uid, itemId));")
    data, err = os.ReadFile("gacha")
    if err != nil && !os.IsNotExist(err) {
        panic(err)
    } else if err == nil {
        var vaChannels map[string]struct{
            Items map[string]int
            Tokens int
            Wait time.Time
        }
        checkFatal(json.Unmarshal(data, &vaChannels))
        fmt.Println("read gacha")
        stmt, _ := db.Prepare("INSERT INTO gachaPlayer VALUES (?, ?, ?);")
        stmt2, _ := db.Prepare("INSERT INTO gachaItems VALUES (?, ?, ?);")
        for k, v := range vaChannels {
            if k == "" {
                continue
            }
            k2, _ := strconv.ParseInt(k, 10, 64)
            if v.Wait.IsZero() {
                stmt.Exec(k2, v.Tokens, nil)
            } else {
                stmt.Exec(k2, v.Tokens, v.Wait)
            }
            for i, s := range v.Items {
                if s <= 0 {
                    continue
                }
                i2, _ := strconv.ParseInt(i, 10, 32)
                stmt2.Exec(k2, i2, s)
            }
        }
        stmt.Close()
        stmt2.Close()
        fmt.Println("inserted gacha")
    }

    db.Exec("CREATE TABLE kekGuilds (gid UNSIGNED BIGINT PRIMARY KEY);")
    db.Exec("CREATE TABLE kekUsers (uid UNSIGNED BIGINT PRIMARY KEY, score INTEGER);")
    db.Exec("CREATE TABLE kekMsgs (uid UNSIGNED BIGINT REFERENCES kekUsers, mid UNSIGNED BIGINT, score INTEGER DEFAULT 0 NOT NULL, expiry TIMESTAMP NOT NULL, PRIMARY KEY (uid, mid));")
    data, err = os.ReadFile("kek")
    if err != nil && !os.IsNotExist(err) {
        panic(err)
    } else if err == nil {
        var vaChannels struct{
            Guilds map[string]interface{}
            Users map[string]map[string]int
        }
        checkFatal(json.Unmarshal(data, &vaChannels))
        fmt.Println("read kek")
        stmt, _ := db.Prepare("INSERT INTO kekGuilds VALUES (?);")
        for k, _ := range vaChannels.Guilds {
            k2, _ := strconv.ParseInt(k, 10, 64)
            stmt.Exec(k2)
        }
        stmt.Close()
        stmt, _ = db.Prepare("INSERT INTO kekUsers VALUES (?, ?);")
        stmt2, _ := db.Prepare("INSERT INTO kekMsgs VALUES (?, ?, ?, ?);")
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
                expiry, _ := discordgo.SnowflakeTimestamp(k)
                expiry = expiry.AddDate(0, 0, 4)
                stmt2.Exec(k2, i2, s, expiry)
            }
        }
        stmt.Close()
        stmt2.Close()
        fmt.Println("inserted kek")
    }
}

