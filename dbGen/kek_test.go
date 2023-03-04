package main_test

import "testing"

func BenchmarkKekGuilds(b *testing.B) {
	b.SkipNow()
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

func BenchmarkKekInsert(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	db.Exec("INSERT OR REPLACE INTO kekUsers (uid) VALUES (?);", 300)
	db.Exec("DELETE FROM kekMsgs WHERE uid = ?;", 300)
	stmt, err := db.Prepare("INSERT INTO kekMsgs (uid, mid, score) VALUES (?001, ?002, ?003) ON CONFLICT DO UPDATE SET score=excluded.score;")
	checkFatal(err)
	defer stmt.Close()
	stmt.Exec(300, 1, 2)
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			_, err := stmt.Exec(300, 1, 2)
			checkFatal(err)
		}
	})
}

func BenchmarkKekInsert2(b *testing.B) {
	db := setupHelper(b)
	defer db.Close()
	db.Exec("INSERT OR REPLACE INTO kekUsers (uid) VALUES (?);", 300)
	db.Exec("DELETE FROM kekMsgs2 WHERE uid = ?;", 300)
	stmt, err := db.Prepare("INSERT INTO kekMsgs2 (uid, mid, score) VALUES (?001, ?002, ?003) ON CONFLICT DO UPDATE SET score=excluded.score;")
	checkFatal(err)
	defer stmt.Close()
	stmt.Exec(300, 1, 2)
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			_, err := stmt.Exec(300, 1, 2)
			checkFatal(err)
		}
	})
}

func BenchmarkKekCombine(b *testing.B) {
	b.SkipNow()
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
			_, err = tx.Stmt(stmt).Exec(348976)
			checkFatal(err)
			tx.Rollback()
		}
	})
}

func BenchmarkKekCombine2(b *testing.B) {
	b.SkipNow()
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
			_, err = tx.Stmt(stmt).Exec(348976)
			checkFatal(err)
			tx.Rollback()
		}
	})
}
