package main_test

import (
	"database/sql"
	"testing"
)

func BenchmarkGacha(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT itemId, count FROM gachaItems WHERE uid=?001 AND count > 0 ORDER BY itemId LIMIT 100 OFFSET ?002 * 20;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkGachaItem", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res, err := stmt.Query(210556673188823041, 0)
				if err != nil {
					b.Fatal(err)
				}
				res.Close()
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("SELECT count FROM gachaItems WHERE uid=?001 AND itemId=?002;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkGachaItemById", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(210556673188823041, 1)
				err := res.Scan(&sql.NullInt32{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT INTO gachaItems (uid, itemId, count) VALUES (?001, ?002, 1) ON CONFLICT DO UPDATE SET count = count + 1;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkGachaPull", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, 777)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("UPDATE gachaItems SET count = count - ?003 WHERE uid=?001 AND itemID=?002;")
	checkFatal(err)
	defer stmt.Close()
	stmt2, err := db.Prepare("INSERT INTO gachaItems (uid, itemId, count) VALUES (?001, ?002, ?003) ON CONFLICT DO UPDATE SET count = count + ?003;")
	checkFatal(err)
	defer stmt2.Close()
	b.Run("BenchmarkGachaTransfer", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				tx, err := db.Begin()
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt).Exec(1234, 777, 1)
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt2).Exec(1235, 777, 1)
				if err != nil {
					b.Fatal(err)
				}
				tx.Commit()
			}
		})
	})
	db.Exec("DELETE FROM gachaItems WHERE uid = ?001 OR uid = ?002;", 1234, 1235)
}

func BenchmarkGacha2(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT itemId, count FROM gachaItems2 WHERE uid=?001 AND count > 0 ORDER BY itemId LIMIT 100 OFFSET ?002 * 20;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkGachaItem", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res, err := stmt.Query(210556673188823041, 0)
				if err != nil {
					b.Fatal(err)
				}
				res.Close()
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("SELECT count FROM gachaItems2 WHERE uid=?001 AND itemId=?002;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkGachaItemById", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				res := stmt.QueryRow(210556673188823041, 1)
				err := res.Scan(&sql.NullInt32{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("INSERT INTO gachaItems2 (uid, itemId, count) VALUES (?001, ?002, 1) ON CONFLICT DO UPDATE SET count = count + 1;")
	checkFatal(err)
	defer stmt.Close()
	b.Run("BenchmarkGachaPull", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				_, err := stmt.Exec(1234, 777)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
	stmt.Close()
	stmt, err = db.Prepare("UPDATE gachaItems2 SET count = count - ?003 WHERE uid=?001 AND itemID=?002;")
	checkFatal(err)
	defer stmt.Close()
	stmt2, err := db.Prepare("INSERT INTO gachaItems2 (uid, itemId, count) VALUES (?001, ?002, ?003) ON CONFLICT DO UPDATE SET count = count + ?003;")
	checkFatal(err)
	defer stmt2.Close()
	b.Run("BenchmarkGachaTransfer", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				tx, err := db.Begin()
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt).Exec(1234, 777, 1)
				if err != nil {
					b.Fatal(err)
				}
				_, err = tx.Stmt(stmt2).Exec(1235, 777, 1)
				if err != nil {
					b.Fatal(err)
				}
				tx.Commit()
			}
		})
	})
	db.Exec("DELETE FROM gachaItems2 WHERE uid = ?001 OR uid = ?002;", 1234, 1235)
}
