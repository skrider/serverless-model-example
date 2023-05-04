package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB
var once sync.Once

func GetDB() *sql.DB {
	once.Do(func() {
		var err error

		db, err = connect()
		if err != nil {
			panic(err)
		}

		err = prepare(db)
		if err != nil {
			panic(err)
		}

		for i := 0; i < 30; i++ {
			err := db.Ping()
			if err == nil {
				break
			}
			time.Sleep(time.Second)
		}
	})
	if db == nil {
		panic("db is nil")
	}
	return db
}

func connect() (*sql.DB, error) {
	bin, err := ioutil.ReadFile("/run/secrets/db-password")
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("postgres://postgres:%s@db:5432/example?sslmode=disable", strings.Trim(string(bin), "\n"))
	return sql.Open("postgres", url)
}

func prepare(db *sql.DB) error {
	if _, err := db.Exec("DROP TABLE IF EXISTS jobs"); err != nil {
		return err
	}

	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS jobs (id SERIAL, title VARCHAR)"); err != nil {
		return err
	}

	for i := 0; i < 5; i++ {
		if _, err := db.Exec("INSERT INTO blog (title) VALUES ($1);", fmt.Sprintf("Blog post #%d", i)); err != nil {
			return err
		}
	}
	return nil
}

