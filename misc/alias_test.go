package main_test

import (
	"database/sql"
	"testing"
)

func BenchmarkAlias(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT key FROM songAlias WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasList", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res, err := stmt.Query(382043119157510155)
				if err != nil {
					b.Fatal(err)
				}
				res.Close()
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("SELECT value FROM songAlias WHERE gid=?001 AND key=?002;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasLookup", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(382043119157510155, "titanic")
				err := res.Scan(&sql.NullString{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT OR REPLACE INTO songAlias (gid, key, value) VALUES (?001, ?002, ?003);")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasInsert", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, "test", "this could be a url")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("DELETE FROM songAlias WHERE gid = ?001 AND key = ?002")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasDelete", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, "test")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	db.Exec("DELETE FROM songAlias WHERE gid = ?001;", 1234)
}

func BenchmarkAlias2(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT key FROM songAlias2 WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasList", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res, err := stmt.Query(382043119157510155)
				if err != nil {
					b.Fatal(err)
				}
				res.Close()
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("SELECT value FROM songAlias2 WHERE gid=?001 AND key=?002;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasLookup", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(382043119157510155, "titanic")
				err := res.Scan(&sql.NullString{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT OR REPLACE INTO songAlias2 (gid, key, value) VALUES (?001, ?002, ?003);")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasInsert", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, "test", "this could be a url")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("DELETE FROM songAlias2 WHERE gid = ?001 AND key = ?002")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkAliasDelete", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, "test")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	db.Exec("DELETE FROM songAlias2 WHERE gid = ?001;", 1234)
}
