package main_test

import "testing"

func BenchmarkKekGuilds(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT * FROM kekGuilds WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			stmt.Exec(1234)
		}
	})
}

func BenchmarkKekGuilds2(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare("SELECT * FROM kekGuilds2 WHERE gid=?001;")
	checkFatal(err)
	defer stmt.Close()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			stmt.Exec(1234)
		}
	})
}

func BenchmarkKekCombine(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare(`UPDATE kekUsers SET score = score + m.total FROM (
		SELECT uid, SUM(score) total FROM kekMsgs
		WHERE mid < ?001
		GROUP BY uid
	) m WHERE m.uid = kekUsers.uid;
	DELETE FROM kekMsgs WHERE mid < ?001;`)
	checkFatal(err)
	defer stmt.Close()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			tx, err := db.Begin()
			checkFatal(err)
			tx.Stmt(stmt).Exec(348976)
			tx.Rollback()
		}
	})
}

func BenchmarkKekCombine2(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	stmt, err := db.Prepare(`UPDATE kekUsers SET score = score + m.total FROM (
		SELECT uid, SUM(score) total FROM kekMsgs2
		WHERE mid < ?001
		GROUP BY uid
	) m WHERE m.uid = kekUsers.uid;
	DELETE FROM kekMsgs2 WHERE mid < ?001;`)
	checkFatal(err)
	defer stmt.Close()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			tx, err := db.Begin()
			checkFatal(err)
			tx.Stmt(stmt).Exec(348976)
			tx.Rollback()
		}
	})
}
