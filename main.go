package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/russross/blackfriday"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const (
	maxTitleLength = 50
	urlDateLayout  = "20060102150405"
)

var (
	globalConfiguration       *config
	layoutTemplate            *template.Template
	indexPageMarkdownTemplate *template.Template
)

type config struct {
	DBPath         string
	StaticFilePath string
}

func main() {
	var dbPath string
	var needInit bool
	var templatePath string

	flag.StringVar(&dbPath, "db", "sly.db", "SQLite3 DB file path")
	flag.BoolVar(&needInit, "init", false, "DDL execute for db")
	flag.StringVar(&templatePath, "t", "", "customize template file path")
	flag.Parse()

	globalConfiguration = &config{DBPath: dbPath, StaticFilePath: "./static"}

	if needInit == true {
		if err := execDDL(); err != nil {
			log.Fatal(err)
		}
		log.Printf("DB: %s initialized\n", globalConfiguration.DBPath)
		os.Exit(0)
	}

	if b, err := ioutil.ReadFile(templatePath); err != nil {
		layoutTemplate = template.Must(template.New("layout").Parse(layoutTemplateText))
	} else {
		layoutTemplate = template.Must(template.New("layout").Parse(string(b)))
	}
	indexPageMarkdownTemplate = template.Must(template.New("index").Parse(indexPageMarkdownTemplateText))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/index.html", indexHandler)
	http.HandleFunc("/profile", profileHandlerPortal)
	http.HandleFunc("/articles/", articlesHandlerPortal)
	http.HandleFunc("/static/", staticFileHandlerPortal)

	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

func staticFileHandlerPortal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		staticFileGetHandler(w, r)
	case "PUT":
		staticFilePutHandler(w, r)
	default:
		http.NotFound(w, r)
	}
}

func staticFileGetHandler(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/static/")
	http.ServeFile(w, r, p)
}

func staticFilePutHandler(w http.ResponseWriter, r *http.Request) {
	// 投稿内容を取得
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}

	// パスを構成
	p := strings.TrimPrefix(r.URL.Path, "/static/")
	p = filepath.Clean(p)
	p = filepath.Join(globalConfiguration.StaticFilePath, p)

	// 書き込み
	err = ioutil.WriteFile(p, b, os.ModePerm)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// レスポンス
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: .json, .mdの対応
	articles, err := selectMultiArticles()
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

	ren := &renderer{
		Title:    "index",
		Contents: string(c),
	}
	renderbuf := bytes.NewBufferString("")
	if err = layoutTemplate.Execute(renderbuf, ren); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(renderbuf.Bytes()))
}

func profileHandlerPortal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		profileGetHandler(w, r)
	case "PUT":
		profilePutHandler(w, r)
	default:
		http.NotFound(w, r)
	}
}

func profileGetHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: .mdの対応
	pro, exist, err := selectProfile()
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}

	var c []byte
	if exist == true {
		c = blackfriday.MarkdownCommon([]byte(pro.Contents))
	}

	ren := &renderer{
		Title:    "profile",
		Contents: string(c),
	}
	renderbuf := bytes.NewBufferString("")
	if err = layoutTemplate.Execute(renderbuf, ren); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(renderbuf.Bytes()))
}

func profilePutHandler(w http.ResponseWriter, r *http.Request) {
	// 投稿内容を取得
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}

	// DB保存
	p := &profile{
		Contents: string(b),
	}
	if err := p.save(); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// make response
	w.WriteHeader(http.StatusCreated)
	res := &responseJSON{
		URL:       "/profile.html",
		Title:     "profile",
		Contents:  p.Contents,
		CreatedAt: time.Now(),
	}
	resb, err := json.Marshal(res)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	fmt.Fprintf(w, string(resb))
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
	a, exist, err := selectArticle(name)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	if exist == false {
		http.NotFound(w, r)
		return
	}
	switch ext {
	case "html":
		out := blackfriday.MarkdownCommon([]byte(a.Contents))

		ren := &renderer{
			Title:    a.Title,
			Contents: string(out),
		}
		renderbuf := bytes.NewBufferString("")
		if err = layoutTemplate.Execute(renderbuf, ren); err != nil {
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
	a, exist, err := selectArticle(name)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	if exist == false {
		http.NotFound(w, r)
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
	if err := a.update(); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// make response
	w.WriteHeader(http.StatusOK)
	res := &responseJSON{
		URL:       a.URL(),
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
	a := &article{
		Title:     t,
		Contents:  c,
		CreatedAt: time.Now(),
	}
	if err := a.insert(); err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// make response
	w.WriteHeader(http.StatusCreated)
	res := &responseJSON{
		URL:       a.URL(),
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
	a, exist, err := selectArticle(name)
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	if exist == false {
		http.NotFound(w, r)
		return
	}
	err = deleteArticle(a.urlDateString())
	if err != nil {
		log.Println(err)
		errorPageRender(w, r)
		return
	}
	// make response
	w.WriteHeader(http.StatusOK)
	res := &responseJSON{
		URL:       a.URL(),
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
