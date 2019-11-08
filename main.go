package main

// SPDX-License-Identifier: GPL-3.0-only
//
// Copyright (C) 2019 cmr@informatik.wtf
// This file is part of rmb.
//
// rmb is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// rmb is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with rmb. If not, see <https://www.gnu.org/licenses/>.

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"text/template"
	"time"

	"github.com/Depado/bfchroma"
	"github.com/alecthomas/chroma"
	"github.com/cmr-informatik/front"
	bf "github.com/russross/blackfriday/v2"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"gopkg.in/yaml.v2"
)

const (
	srcDir        = "src"
	imgDir        = "img"
	cssDir        = "css"
	fontsDir      = "fonts"
	tmpDir        = "tmp"
	tmpPage       = "page"
	dPerms        = 0755
	fPerms        = 0644
	yamlDecodeErr = "failed to decode YAML from '%s'\n"
	yamlEncodeErr = "failed to encode YAML into '%s'\n"
)

var renderer = bfchroma.NewRenderer()
var minifier = minify.New()

func init() {
	minifier.AddFunc("text/html", html.Minify)
	minifier.Add("text/html", &html.Minifier{KeepDocumentTags: true})
	minifier.AddFunc("text/css", css.Minify)
}

func loadChromaStyle(f string) *chroma.Style {
	s := struct {
		Name   string
		Styles map[string]string
	}{}

	data, err := ioutil.ReadFile(f)
	if perr(err, "") {
		return nil
	}

	err = yaml.UnmarshalStrict(data, &s)
	if perr(err, yamlDecodeErr, f) {
		return nil
	}

	var styles = make(chroma.StyleEntries)
	for k, v := range s.Styles {
		switch k {
		case "keyword":
			styles[chroma.Keyword] = v
		case "name":
			styles[chroma.Name] = v
		case "literal":
			styles[chroma.Literal] = v
		case "string":
			styles[chroma.String] = v
		case "number":
			styles[chroma.Number] = v
		case "operator":
			styles[chroma.Operator] = v
		case "punctuation":
			styles[chroma.Punctuation] = v
		case "comment":
			styles[chroma.Comment] = v
		case "normal":
			styles[chroma.Background] = v
		default:
			stderr("invalid chroma-style option '%s'\n", k)
			return nil
		}
	}

	return chroma.MustNewStyle(s.Name, styles)
}

type conf struct {
	Name string
	CSS  []string `yaml:"css-merge-order"`
	Out  string
	In   string
	Sty  string `yaml:"chroma-style"`
}

func (c *conf) load(f string) {
	data, err := ioutil.ReadFile(f)
	kill(err, "")

	err = yaml.UnmarshalStrict(data, &c)
	kill(err, yamlDecodeErr, f)

	err = os.MkdirAll(c.Out, dPerms)
	kill(err, "")

	dirs := []string{srcDir, imgDir, cssDir, fontsDir, tmpDir}
	for _, d := range dirs {
		err := os.MkdirAll(filepath.Join(c.In, d), dPerms)
		kill(err, "")

		if d == imgDir || d == fontsDir {
			err := os.MkdirAll(filepath.Join(c.Out, d), dPerms)
			kill(err, "")
		}
	}

	if c.Sty != "" {
		s := loadChromaStyle(c.Sty)
		if s != nil {
			renderer.Style = s
		}
	}
}

type indexElem struct {
	Title string
	Added int64
	Date  string
	Link  string
}

type page struct {
	Title      string
	Added      int64
	Index      bool
	IndexElems []*indexElem `yaml:"-"`
	Body       []byte       `yaml:"-"`
	Filebase   string
}

func (p *page) load(f string) (err error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return
	}

	body, err := front.Unmarshal(data, &p)
	if err != nil {
		stderr(yamlDecodeErr, f)
		return
	}

	p.Body = bf.Run(body, bf.WithRenderer(renderer))
	return
}

func (p *page) save(f string) (err error) {
	data, err := front.Marshal(p, []byte(p.Body))
	if err != nil {
		stderr(yamlEncodeErr, f)
		return
	}

	err = ioutil.WriteFile(f, data, fPerms)
	return
}

func (p *page) render(dir string, tmp *template.Template) (err error) {
	buf := new(bytes.Buffer)
	err = tmp.ExecuteTemplate(buf, tmpPage, p)
	if err != nil {
		return
	}

	body, err := minifier.Bytes("text/html", buf.Bytes())
	if err != nil {
		return
	}

	f := filepath.Join(dir, p.Filebase+".html")
	err = ioutil.WriteFile(f, body, fPerms)
	return
}

func mergeCSS(c *conf, wg *sync.WaitGroup) {
	var sty []byte
	for _, x := range c.CSS {
		f := filepath.Join(c.In, cssDir, x)
		data, err := ioutil.ReadFile(f)
		if perr(err, "") {
			wg.Done()
			return
		}
		sty = append(sty, data...)
	}

	sty, err := minifier.Bytes("text/css", sty)
	if perr(err, "") {
		wg.Done()
		return
	}

	f := filepath.Join(c.Out, "all.css")
	err = ioutil.WriteFile(f, sty, fPerms)
	perr(err, "")
	wg.Done()
}

type renderFuncArgs struct {
	c     *conf
	tmp   *template.Template
	index chan *page
	posts chan *indexElem
	wg    *sync.WaitGroup
}

func renderPage(f string, args *renderFuncArgs) {
	var p page
	err := p.load(f)
	if perr(err, "") {
		args.wg.Done()
		return
	}

	if p.Index {
		args.index <- &p
		args.wg.Done()
		return
	}

	err = p.render(args.c.Out, args.tmp)
	if perr(err, "") {
		args.wg.Done()
		return
	}

	args.posts <- &indexElem{Title: p.Title, Added: p.Added,
		Link: p.Filebase + ".html"}
	args.wg.Done()
}

func renderIndex(args *renderFuncArgs) {
	idx := <-args.index
	nposts := len(args.posts)
	for i := 0; i < nposts; i++ {
		idx.IndexElems = append(idx.IndexElems, <-args.posts)

		elem := idx.IndexElems[i]
		date := time.Unix(elem.Added, 0)
		elem.Date = fmt.Sprintf("%02d%02d%02d: ", date.Month(),
			date.Day(), date.Year())
	}

	sort.SliceStable(idx.IndexElems, func(i int, j int) bool {
		return idx.IndexElems[i].Added >= idx.IndexElems[j].Added
	})

	err := idx.render(args.c.Out, args.tmp)
	perr(err, "")
}

func compile(c *conf) {
	funcsmap := template.FuncMap{"tostring": tostring}
	pageTmpF := filepath.Join(c.In, tmpDir, tmpPage)
	tmp, err := template.New("").Funcs(funcsmap).ParseFiles(pageTmpF)
	kill(err, "error loading template from '%s'\n", pageTmpF)

	src, err := filepath.Glob(filepath.Join(c.In, srcDir, "*.md"))
	kill(err, "error finding markdown pages\n")

	var wg sync.WaitGroup

	npages := len(src)
	if npages <= 0 {
		stderr("could not find any pages, try 'rmb <conf> page'\n")
		os.Exit(1)
	}

	posts := make(chan *indexElem, npages-1)
	index := make(chan *page, 1)
	// Bundle arguments for renderPage and renderIndex
	args := renderFuncArgs{c: c, tmp: tmp, index: index, posts: posts,
		wg: &wg}

	// Pages (index + posts)
	wg.Add(npages)
	for _, s := range src {
		go renderPage(s, &args)
	}

	// CSS
	wg.Add(1)
	go mergeCSS(c, &wg)

	// Images, fonts
	dirs := []string{imgDir, fontsDir}
	wg.Add(len(dirs))
	for _, d := range dirs {
		x := d
		go func() {
			from := filepath.Join(c.In, x)
			dest := filepath.Join(c.Out, x)
			dircpy(from, dest)
			wg.Done()
		}()
	}

	wg.Wait()

	// Index
	renderIndex(&args)
}

func main() {
	args := os.Args[1:]
	if len(args) < 1 || args[0] == "-h" {
		fmt.Println(usage)
		os.Exit(1)
	}

	var c conf
	c.load(args[0])

	if len(args) == 1 {
		compile(&c)
		os.Exit(0)
	}

	switch args[1] {
	case "page":
		var p page
		scanner := bufio.NewScanner(os.Stdin)
		stdout("index (y/n): ")
		scanner.Scan()
		ans := scanner.Text()
		if ans == "y" {
			p.Title = c.Name
			p.Index = true
			p.Filebase = "index"
		} else {
			stdout("title (txt): ")
			scanner.Scan()
			p.Title = scanner.Text()
			p.Added = time.Now().Unix()
			p.Filebase = unixfy(p.Title)
		}
		f := filepath.Join(c.In, srcDir, p.Filebase+".md")
		p.save(f)
		stdout("created new post '%s'\n", f)
	default:
		stderr("invalid command '%s'\n", args[1])
		os.Exit(1)
	}
}
