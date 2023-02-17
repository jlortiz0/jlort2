package main_test

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

func checkFatal(err error) {
	if err != nil {
		panic(err)
	}
}

func setupHelper(b *testing.B) *sql.DB {
	b.Helper()
	cwd, err := os.Getwd()
	checkFatal(err)
	if strings.HasSuffix(cwd, "misc") {
		checkFatal(os.Chdir(".."))
	}
	db, err := sql.Open("sqlite3", "persistent2.db")
	checkFatal(err)
	return db
}

func BenchmarkQuotes(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT COUNT(*) FROM quotes WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkQuotesLen", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(382043119157510155)
				err := res.Scan(&sql.NullInt32{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("SELECT quote FROM quotes WHERE gid=?001 AND ind=?002;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkQuotesByInd", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(382043119157510155, 2)
				err := res.Scan(&sql.NullString{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT INTO quotes (gid, ind, quote) SELECT ?001, COUNT(*) + 1, ?002 FROM quotes WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkQuotesInsert", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, "test quote")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT OR REPLACE INTO quotes SELECT ?001, ind - 1, quote FROM quotes WHERE gid=?001 AND ind > ?002 ORDER BY ind ASC;")
	checkFatal(err)
	defer stmt.Close()
	stmt2, err := db.Prepare("DELETE FROM quotes WHERE gid = ?001 AND ind = (SELECT COUNT(*) FROM quotes WHERE gid = ?001);")
	checkFatal(err)
	defer stmt2.Close()
	b.Run("BenchmarkQuotesDelete", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				tx, err := db.Begin()
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt).Exec(1234, 1)
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt2).Exec(1234)
				if err != nil {
					b.Fatal(err)
				}
				tx.Commit()
			}
		})
	})
	db.Exec("DELETE FROM quotes WHERE gid = ?001;", 1234)
}

func BenchmarkQuotes2(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT COUNT(*) FROM quotes2 WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkQuotesLen", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(382043119157510155)
				err := res.Scan(&sql.NullInt32{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("SELECT quote FROM quotes2 WHERE gid=?001 AND ind=?002;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkQuotesByInd", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(382043119157510155, 2)
				err := res.Scan(&sql.NullString{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT INTO quotes2 (gid, ind, quote) SELECT ?001, COUNT(*) + 1, ?002 FROM quotes2 WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkQuotesInsert", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, "test quote")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT OR REPLACE INTO quotes2 SELECT ?001, ind - 1, quote FROM quotes2 WHERE gid=?001 AND ind > ?002 ORDER BY ind ASC;")
	checkFatal(err)
	defer stmt.Close()
	stmt2, err := db.Prepare("DELETE FROM quotes2 WHERE gid = ?001 AND ind = (SELECT COUNT(*) FROM quotes2 WHERE gid = ?001);")
	checkFatal(err)
	defer stmt2.Close()
	b.Run("BenchmarkQuotesDelete", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				tx, err := db.Begin()
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt).Exec(1234, 1)
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt2).Exec(1234)
				if err != nil {
					b.Fatal(err)
				}
				tx.Commit()
			}
		})
	})
	db.Exec("DELETE FROM quotes2 WHERE gid = ?001;", 1234)
}
