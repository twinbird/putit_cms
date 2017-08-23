package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"time"
)

type article struct {
	Title     string
	Contents  string
	CreatedAt time.Time
}

func (art *article) urlDateString() string {
	return art.CreatedAt.Format(urlDateLayout)
}

func (art *article) URL() string {
	return "/articles/" + art.urlDateString() + ".html"
}

func (art *article) insert() error {
	db, err := dbOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO articles(title, contents, created_at) VALUES(?, ?, ?)
	`, art.Title, art.Contents, art.urlDateString())
	if err != nil {
		return err
	}
	return nil
}

func (art *article) update() error {
	db, err := dbOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		update
			articles
		SET
			title = ?,
			contents = ?
		WHERE
			created_at = ?
	`, art.Title, art.Contents, art.urlDateString())
	if err != nil {
		return err
	}
	return nil
}

func deleteArticle(dateStr string) error {
	db, err := dbOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		DELETE
		FROM
			articles
		WHERE
			created_at = ?
	`, dateStr)
	if err != nil {
		return err
	}
	return nil
}

func selectMultiArticles() ([]*article, error) {
	db, err := dbOpen()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT
			a.title AS title,
			a.contents AS contents,
			a.created_at AS created_at
		FROM
			articles a
	`)
	if err != nil {
		return nil, err
	}
	var articles []*article
	for rows.Next() {
		var art article
		var t string
		err := rows.Scan(&(art.Title), &(art.Contents), &t)
		if err != nil {
			return nil, err
		}
		art.CreatedAt, err = time.Parse(urlDateLayout, t)
		if err != nil {
			return nil, err
		}
		articles = append(articles, &art)
	}

	return articles, nil
}

func selectArticle(dateStr string) (*article, bool, error) {
	db, err := dbOpen()
	if err != nil {
		return nil, false, err
	}
	defer db.Close()

	var art article
	err = db.QueryRow(`
		SELECT
			a.title AS title,
			a.contents AS contents
		FROM
			articles a
		WHERE
			a.created_at = ?
	`, dateStr).Scan(&(art.Title), &(art.Contents))
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	art.CreatedAt, err = time.Parse(urlDateLayout, dateStr)
	if err != nil {
		return nil, false, err
	}

	return &art, true, nil
}
