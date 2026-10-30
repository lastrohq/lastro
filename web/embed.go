// Package web embute o painel (React, em web/src) no binário do Vintém.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

const semBuild = `<!doctype html><html lang="pt-BR"><meta charset="utf-8"><title>Vintém</title>
<body style="font-family:system-ui;max-width:560px;margin:15vh auto;padding:0 24px;line-height:1.5">
<h1>Painel não incluído neste binário</h1>
<p>Este binário foi compilado sem o painel (por exemplo, com <code>go install</code>).
A API funciona normalmente. Para ter o painel, use a imagem Docker ou um binário dos
<a href="https://github.com/vintemhq/vintem/releases">releases</a>, ou compile:</p>
<pre>cd web && npm ci && npm run build && cd .. && go build ./cmd/vintem</pre></body></html>`

// Painel serve o build em /painel/. Os assets têm hash no nome e ficam em
// cache para sempre; o index.html nunca fica em cache.
func Painel() http.Handler {
	sub, err := fs.Sub(dist, "dist/painel")
	if err != nil {
		panic(err) // "dist/painel" é um caminho fixo e válido
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return seguro(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(semBuild))
		}))
	}
	arquivos := http.StripPrefix("/painel/", http.FileServerFS(sub))
	return seguro(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/painel/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		arquivos.ServeHTTP(w, r)
	}))
}

// seguro põe os cabeçalhos de segurança do painel. O React usa estilo inline,
// por isso style-src aceita 'unsafe-inline'; script só do próprio Vintém.
func seguro(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		h.ServeHTTP(w, r)
	})
}
