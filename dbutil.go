package main

import (
	"database/sql"
)

func dbOpen() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", globalConfiguration.DBPath)
	if err != nil {
		return nil, err
	}
	return db, err
}

func execDDL() error {
	db, err := dbOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
CREATE TABLE articles (
	created_at VARCHAR(14) PRIMARY KEY,
	title VARCHAR(50) NOT NULL,
	contents VARCHAR(50000) NOT NULL
)
	`)
	if err != nil {
		return err
	}
	return nil
}
