package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/russross/blackfriday"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"
)

const (
	maxTitleLength     = 50
	URLDateLayout      = "20060102150405"
	layoutTemplateText = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>{{.SiteName}} - {{.Title}}</title>
<link rel="stylesheet" href="css/styles.css">
</head>
<body>
{{.Contents}}
</body>
</html>`

	indexPageMarkdownTemplateText = `{{range .}}
* {{.Title}} - {{.CreatedAt}}
{{end}}`
)

var (
	globalConfiguration       *Config
	layoutTemplate            *template.Template
	indexPageMarkdownTemplate *template.Template
)

type Renderer struct {
	SiteName string
	Title    string
	Contents string
}

type ResponseJSON struct {
	URL       string
	Title     string
	Contents  string
	CreatedAt time.Time
}

type Config struct {
	DBPath   string
	SiteName string
}

type Article struct {
	Title     string
	Contents  string
	CreatedAt time.Time
}

func (art *Article) URLDateString() string {
	return art.CreatedAt.Format(URLDateLayout)
}

func DBOpen() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", globalConfiguration.DBPath)
	if err != nil {
		return nil, err
	}
	return db, err
}

func execDDL() error {
	db, err := DBOpen()
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

func (art *Article) Insert() error {
	db, err := DBOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO articles(title, contents, created_at) VALUES(?, ?, ?)
	`, art.Title, art.Contents, art.URLDateString())
	if err != nil {
		return err
	}
	return nil
}

func (art *Article) Update() error {
	db, err := DBOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		UPDATE
			articles
		SET
			title = ?,
			contents = ?
		WHERE
			created_at = ?
	`, art.Title, art.Contents, art.URLDateString())
	if err != nil {
		return err
	}
	return nil
}

func DeleteArticle(dateStr string) error {
	db, err := DBOpen()
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

func SelectMultiArticles() ([]*Article, error) {
	db, err := DBOpen()
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
	var articles []*Article
	for rows.Next() {
		var art Article
		var t string
		err := rows.Scan(&(art.Title), &(art.Contents), &t)
		if err != nil {
			return nil, err
		}
		art.CreatedAt, err = time.Parse(URLDateLayout, t)
		if err != nil {
			return nil, err
		}
		articles = append(articles, &art)
	}

	return articles, nil
}

func SelectArticle(dateStr string) (*Article, error) {
	db, err := DBOpen()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var art Article
	err = db.QueryRow(`
		SELECT
			a.title AS title,
			a.contents AS contents
		FROM
			articles a
		WHERE
			a.created_at = ?
	`, dateStr).Scan(&(art.Title), &(art.Contents))
	if err != nil {
		return nil, err
	}
	art.CreatedAt, err = time.Parse(URLDateLayout, dateStr)
	if err != nil {
		return nil, err
	}

	return &art, nil
}

func main() {
	var dbPath string
	var needInit bool

	flag.StringVar(&dbPath, "db", "sly.db", "SQLite3 DB file path")
	flag.BoolVar(&needInit, "init", false, "DDL execute for db")
	flag.Parse()

	globalConfiguration = &Config{DBPath: dbPath, SiteName: "test"}

	if needInit == true {
		if err := execDDL(); err != nil {
			log.Fatal(err)
		}
		log.Printf("DB: %s initialized\n", globalConfiguration.DBPath)
	}

	layoutTemplate = template.Must(template.New("layout").Parse(layoutTemplateText))
	indexPageMarkdownTemplate = template.Must(template.New("index").Parse(indexPageMarkdownTemplateText))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/index.html", indexHandler)
	http.HandleFunc("/articles/", articlesHandlerPortal)

	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatal(err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: .json, .mdの対応
	articles, err := SelectMultiArticles()
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}

	buf := bytes.NewBufferString("")
	if err := indexPageMarkdownTemplate.Execute(buf, articles); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	c := blackfriday.MarkdownCommon(buf.Bytes())

	renderer := &Renderer{
		SiteName: globalConfiguration.SiteName,
		Title:    "index",
		Contents: string(c),
	}
	renderbuf := bytes.NewBufferString("")
	if err = layoutTemplate.Execute(renderbuf, renderer); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(renderbuf.Bytes()))
}

func articlesHandlerPortal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		articlesGetHandler(w, r)
	case "POST":
		articlesPostHandler(w, r)
	case "PUT":
		articlesPutHandler(w, r)
	case "DELETE":
		articlesDeleteHandler(w, r)
	default:
		http.NotFound(w, r)
	}
}

// return value: filename, ext, error
func urlFileName(url string) (string, string, error) {
	// index.html/ <- wrong. trim
	url = strings.TrimRight(url, "/")

	// /articles/20170821010101.html
	// -> /, articles, 20...html
	pathes := strings.Split(url, "/")

	// 20170821010101.html
	file := pathes[len(pathes)-1]

	// 20170821010101, html
	r := strings.Split(file, ".")
	if len(r) != 2 {
		return "", "", fmt.Errorf("%s is wrong file name", file)
	}
	return r[0], r[1], nil
}

func articlesGetHandler(w http.ResponseWriter, r *http.Request) {
	name, ext, err := urlFileName(r.URL.Path)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	a, err := SelectArticle(name)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	switch ext {
	case "html":
		out := blackfriday.MarkdownCommon([]byte(a.Contents))

		renderer := &Renderer{
			SiteName: globalConfiguration.SiteName,
			Title:    "index",
			Contents: string(out),
		}
		renderbuf := bytes.NewBufferString("")
		if err = layoutTemplate.Execute(renderbuf, renderer); err != nil {
			log.Println(err)
			errorPageRender(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, string(renderbuf.Bytes()))

	case "md":
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, a.Contents)
	default:
		http.NotFound(w, r)
	}
}

func articlesPutHandler(w http.ResponseWriter, r *http.Request) {
	// 更新対象が存在するか確認
	name, _, err := urlFileName(r.URL.Path)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	a, err := SelectArticle(name)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// 投稿内容を取得
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	c := string(b)
	t, err := titleFromContents(c)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// DB保存
	a.Title = t
	a.Contents = c
	if err := a.Update(); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// make response
	w.WriteHeader(http.StatusOK)
	res := &ResponseJSON{
		URL:       "/articles/" + a.URLDateString() + ".html",
		Title:     a.Title,
		Contents:  a.Contents,
		CreatedAt: a.CreatedAt,
	}
	resb, err := json.Marshal(res)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	fmt.Fprintf(w, string(resb))
}

func articlesPostHandler(w http.ResponseWriter, r *http.Request) {
	// 投稿内容を取得
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	c := string(b)
	t, err := titleFromContents(c)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}

	// DB保存
	a := &Article{
		Title:     t,
		Contents:  c,
		CreatedAt: time.Now(),
	}
	if err := a.Insert(); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// make response
	w.WriteHeader(http.StatusCreated)
	res := &ResponseJSON{
		URL:       "/articles/" + a.URLDateString() + ".html",
		Title:     a.Title,
		Contents:  a.Contents,
		CreatedAt: a.CreatedAt,
	}
	resb, err := json.Marshal(res)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	fmt.Fprintf(w, string(resb))
}

func articlesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	// 更新対象が存在するか確認
	name, _, err := urlFileName(r.URL.Path)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	a, err := SelectArticle(name)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	err = DeleteArticle(a.URLDateString())
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// make response
	w.WriteHeader(http.StatusOK)
	res := &ResponseJSON{
		URL:       "/articles/" + a.URLDateString() + ".html",
		Title:     a.Title,
		Contents:  a.Contents,
		CreatedAt: a.CreatedAt,
	}
	resb, err := json.Marshal(res)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	fmt.Fprintf(w, string(resb))
}

// エラーページを描画する
func errorPageRender(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "Sorry.")
}

// 記事内容からタイトルを取得する
func titleFromContents(contents string) (string, error) {
	// 先頭1行読み出し(UTF-8考慮して4倍取っておく)
	reader := bufio.NewReaderSize(strings.NewReader(contents), 4*maxTitleLength)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	// 行頭に#がついていたら切り落とす
	line = strings.TrimPrefix(line, "# ")
	// 行末に改行文字が残っていたら切り落とす
	line = strings.TrimRight(line, "\n")
	line = strings.TrimRight(line, "\r")

	// TODO:マルチバイト文字対応
	// titleの文字数制限に切り落とす
	if len(line) > 50 {
		line = line[:50]
	}
	return line, nil
}