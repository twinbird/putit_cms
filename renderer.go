package main

import (
	"fmt"
	"net/http"
	"time"
)

const (
	layoutTemplateText = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/2.8.0/github-markdown.min.css">
<style>
	.markdown-body {
		box-sizing: border-box;
		min-width: 200px;
		max-width: 980px;
		margin: 0 auto;
		padding: 45px;
	}

	@media (max-width: 767px) {
		.markdown-body {
			padding: 15px;
		}
	}
</style>
</head>
<body>
<article class="markdown-body">{{.Contents}}</article>
</body>
</html>`

	indexPageMarkdownTemplateText = `<form action="/" method="GET"><input type="text" name="q"><input type="submit" value="search"></form>
{{range .}}
* [{{.Title}}]({{.URL}}) - {{.CreatedAt}}
{{end}}`
)

type renderer struct {
	Title    string
	Contents string
}

type responseJSON struct {
	URL       string
	Title     string
	Contents  string
	CreatedAt time.Time
}

// エラーページを描画する
func errorPageRender(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "Sorry.")
}
