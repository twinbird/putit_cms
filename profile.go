package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type profile struct {
	Contents string
}

func (pro *profile) save() error {
	db, err := dbOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	_, exist, err := selectProfile()
	if err != nil {
		return err
	}
	if exist == false {
		_, err = db.Exec(`INSERT INTO profile(contents) VALUES(?)`, pro.Contents)
		if err != nil {
			return err
		}
	} else {
		_, err = db.Exec(`UPDATE profile SET contents = ?`, pro.Contents)
		if err != nil {
			return err
		}
	}

	return nil
}

func selectProfile() (*profile, bool, error) {
	db, err := dbOpen()
	if err != nil {
		return nil, false, err
	}
	defer db.Close()

	var pro profile
	err = db.QueryRow(`
		SELECT
			p.contents AS contents
		FROM
			profile p
	`).Scan(&(pro.Contents))
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return &pro, true, nil
}
