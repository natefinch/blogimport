/*
 * Copyright (c) 2014, Nate Finch
 * See LICENSE for more information.
 */

package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

type Date time.Time

func (d Date) String() string {
	return time.Time(d).Format("2006-01-02T15:04:05Z")
}

func (d *Date) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var v string
	dec.DecodeElement(&v, &start)
	t, err := time.Parse("2006-01-02T15:04:05.000-07:00", v)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

type Draft bool

func (d *Draft) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var v string
	dec.DecodeElement(&v, &start)
	switch v {
	case "yes":
		*d = true
		return nil
	case "no":
		*d = false
		return nil
	}
	return errors.New("Unknown value for draft boolean: " + v)
}

type AuthorImage struct {
	Src string `xml:"src,attr"`
}

type Reply struct {
	Rel    string `xml:"rel,attr"`
	Link   string `xml:"href,attr"`
	Source string `xml:"source,attr"`
}

type Image struct {
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
	Source string `xml:"src,attr"`
}

type Author struct {
	Name  string `xml:"name"`
	Uri   string `xml:"uri"`
	Image Image  `xml:"image"`
}

type Export struct {
	XMLName xml.Name `xml:"feed"`
	Entries []Entry  `xml:"entry"`
}

type Media struct {
	ThumbnailUrl string `xml:"url,attr"`
}

type Entry struct {
	ID        string  `xml:"id"`
	Published Date    `xml:"published"`
	Updated   Date    `xml:"updated"`
	Draft     Draft   `xml:"control>draft"`
	Title     string  `xml:"title"`
	Content   string  `xml:"content"`
	Tags      Tags    `xml:"category"`
	Author    Author  `xml:"author"`
	Media     Media   `xml:"thumbnail"`
	Source    Reply   `xml:"in-reply-to"`
	Links     []Reply `xml:"link"`
	Reply     uint64
	Children  []int
	Comments  []uint64
	Slug      string
	Extra     string
}

type Tag struct {
	Name   string `xml:"term,attr"`
	Scheme string `xml:"scheme,attr"`
}

type Tags []Tag
type EntrySet []int

func (t Tags) TomlString() string {
	names := []string{}
	for _, t := range t {
		if t.Scheme == "http://www.blogger.com/atom/ns#" {
			names = append(names, strconv.QuoteToGraphic(t.Name))
		}
	}
	return strings.Join(names, ", ")
}

var templ = `+++
title = "{{ escape .Title }}"
{{- if not (eq .Title .Slug) }}
slug = "{{ escape .Slug }}"{{ end }}
date = {{ .Published }}
updated = {{ .Updated }}
{{- with .Tags.TomlString }}
tags = [{{ . }}]{{ end }}
{{- if .Draft }}
draft = true{{ end }}
blogimport = true
{{- with .Extra }}
{{ . }}{{ end }}
{{- if not (len .Comments | eq 0) }}
comments = [ {{range $i, $e := .Comments}}{{if $i}}, {{end}}{{$e}}{{end}} ]{{ end }}
[author]
	name = "{{ .Author.Name }}"
	uri = "{{ .Author.Uri }}"
	image = "{{ .Author.Image.Source }}"
{{- with .Media.ThumbnailUrl }}
[image]
	src = "{{ resizeImage . }}"
	link = ""
	thumblink = "{{ . }}"
	alt = ""
	title = ""
	author = ""
	license = ""
	licenseLink = ""
{{- end }}
+++

{{ .Content }}
`

var t = template.Must(template.New("").Funcs(funcMap).Parse(templ))

// maps the the function into template
var funcMap = template.FuncMap{
	"resizeImage": resizeImage,
	"escape":      escape,
}

// Resize image of thumbnail to larger size (scale to 1600)
func resizeImage(url string) string {
	return strings.Replace(url, "s72-c", "s1600", -1)
}

// Escape the string for use with toml format
func escape(s string) string {
	return strings.Replace(s, "\"", "\\\"", -1)
}

var comtemplate = `id = "{{ .ID }}"
date = {{ .Published }}
updated = {{ .Updated }}
title = '''{{ .Title }}'''
content = '''{{ .Content }}'''
{{- with .Reply }}
reply = {{ . }}{{ end }}
[author]
	name = "{{ .Author.Name }}"
	uri = "{{ .Author.Uri }}"
[author.image]
	source = "{{ .Author.Image.Source }}"
	width = "{{ .Author.Image.Width }}"
	height = "{{ .Author.Image.Height }}"
`

var ct = template.Must(template.New("").Parse(comtemplate))
var exp = Export{}

func (s EntrySet) Len() int {
	return len(s)
}
func (s EntrySet) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s EntrySet) Less(i, j int) bool {
	return time.Time(exp.Entries[s[i]].Published).Before(time.Time(exp.Entries[s[j]].Published))
}

func treeSort(i int) (list []int) {
	sort.Sort(EntrySet(exp.Entries[i].Children))
	for _, v := range exp.Entries[i].Children {
		list = append(list, v)
		list = append(list, treeSort(v)...)
	}
	return
}

var flags = struct {
	Static    string
	Extra     string
	NoBlogger bool
	Comments  bool
	ToMd      bool
	SlugName  bool
}{}

func main() {
	log.SetFlags(0)

	flag.StringVar(&flags.Static, "static", "", "static directory for import images")
	flag.StringVar(&flags.Extra, "extra", "", "additional metadata to set in frontmatter")
	flag.BoolVar(&flags.NoBlogger, "no-blogger", false, "remove blogger specific url")
	flag.BoolVar(&flags.Comments, "comments", false, "import comments")
	flag.BoolVar(&flags.ToMd, "md", false, "convert HTML to markdown")
	flag.BoolVar(&flags.SlugName, "no-slug", true, "-no-slug=false for not use slug as file name")

	flag.Parse()

	if flags.Static != "" {
		if abs, err := filepath.Abs(flags.Static); err == nil {
			flags.Static = abs
		} else {
			log.Fatal(err)
		}
	}

	if flag.NArg() != 2 {
		log.Printf("Usage: %s [options] <xmlfile> <targetdir>", os.Args[0])
		log.Println("options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	dir := flag.Arg(1)

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Join(dir, "comments"), 0755)
	}
	if err != nil {
		log.Fatal(err)
	}

	if !info.IsDir() {
		log.Fatal("Second argument is not a directory.")
	}

	b, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	err = xml.Unmarshal(b, &exp)
	if err != nil {
		log.Fatal(err)
	}

	if len(exp.Entries) < 1 {
		log.Fatal("No blog entries found!")
	}

	postmap := make(map[uint64]int)

	// Go through and create a map of all entries so we can refer to them later by ID number
	for k := range exp.Entries {
		isTemplate := false
		for _, tag := range exp.Entries[k].Tags {
			if tag.Scheme == "http://schemas.google.com/g/2005#kind" {
				switch tag.Name {
				case "http://schemas.google.com/blogger/2008/kind#comment":
					fallthrough
				case "http://schemas.google.com/blogger/2008/kind#post":
				default:
					isTemplate = true
				}
				break
			}
		}
		if isTemplate {
			continue
		}
		if index := strings.LastIndex(exp.Entries[k].ID, "post-"); index >= 0 {
			exp.Entries[k].ID = exp.Entries[k].ID[index+5:]

			if id, err := strconv.ParseUint(exp.Entries[k].ID, 10, 64); err == nil {
				postmap[id] = k
			} else {
				log.Println("Can't parse " + exp.Entries[k].ID)
			}
		}
		for _, link := range exp.Entries[k].Links {
			switch strings.ToLower(link.Rel) {
			case "related":
				exp.Entries[k].Reply, _ = strconv.ParseUint(filepath.Base(link.Link), 10, 64)
			case "alternate":
			case "replies":
				exp.Entries[k].Slug = strings.Replace(filepath.Base(link.Link), filepath.Ext(link.Link), "", -1)
			}
		}
	}

	// Build comment heirarchy
	if flags.Comments {
		for k, entry := range exp.Entries {
			for _, tag := range entry.Tags {
				if tag.Name == "http://schemas.google.com/blogger/2008/kind#comment" &&
					tag.Scheme == "http://schemas.google.com/g/2005#kind" {
					parent := entry.Reply
					if parent == 0 {
						parent, _ = strconv.ParseUint(filepath.Base(entry.Source.Source), 10, 64)
					}
					if parent == 0 {
						log.Println("Skipping deleted comment " + entry.ID)
						break
					}
					if i, ok := postmap[parent]; ok {
						exp.Entries[i].Children = append(exp.Entries[i].Children, k)
					} else {
						panic(strconv.Itoa(k) + " entry did not exist")
					}
					err := writeComment(entry, dir)
					if err != nil {
						log.Println(err)
					}
					break
				}
			}
		}
	}

	count := 0
	drafts := 0
	for k, entry := range exp.Entries {
		isPost := false
		for _, tag := range entry.Tags {
			if tag.Name == "http://schemas.google.com/blogger/2008/kind#post" &&
				tag.Scheme == "http://schemas.google.com/g/2005#kind" {
				isPost = true
				break
			}
		}

		if !isPost {
			continue
		}

		log.Println("Importing post: " + entry.Title)

		// Sort and flatten all top level comment chains
		entry.Children = treeSort(k)
		for _, v := range entry.Children {
			if id, err := strconv.ParseUint(exp.Entries[v].ID, 10, 64); err == nil {
				entry.Comments = append(entry.Comments, id)
			}
		}
		if flags.Extra != "" {
			entry.Extra = flags.Extra
		}

		if flags.NoBlogger {
			if strings.Contains(entry.Author.Uri, "blogger.com") {
				entry.Author.Uri = ""
			}

			if strings.Contains(entry.Author.Image.Source, "blogger.com") {
				entry.Author.Image.Source = ""
			}
		}

		if flags.Static != "" {
			entry.Content = imgToLocal(entry.Content, flags.Static)
		}

		if flags.ToMd {
			convert := md.NewConverter("", true, nil)
			entry.Content, err = convert.ConvertString(entry.Content)
			if err != nil {
				log.Fatal(err)
			}
		}

		entry.Content = strings.Replace(entry.Content, "&nbsp;", " ", -1)
		if err := writeEntry(entry, dir); err != nil {
			log.Fatalf("Failed writing post %q to disk:\n%s", entry.Title, err)
		}
		if entry.Draft {
			drafts++
		} else {
			count++
		}
	}
	log.Printf("Wrote %d published posts to disk.", count)
	log.Printf("Wrote %d drafts to disk.", drafts)
}

// imgToLocal for each image, download it with downloadImage and replace the src with the local path
func imgToLocal(content, dir string) string {
	re := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*>`)
	for _, match := range re.FindAllStringSubmatch(content, -1) {
		if len(match) != 2 {
			continue
		}

		src := match[1]

		if !strings.HasPrefix(src, "http") {
			continue
		}

		img, err := downloadImage(src, dir)
		if err != nil {
			log.Printf("Failed to download image %q: %s", src, err)
			continue
		}

		content = strings.Replace(content, src, img, 1)
	}

	return content
}

// downloadImage downloads an image from the given URL and saves it to the given directory
// Returns the img path part after the static directory (e.g. /img/foo.jpg) and an error
func downloadImage(uri, dir string) (string, error) {
	// Doanload the image
	resp, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("Error " + resp.Status)
	}

	// Create the image file if it doesn't exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	// Build the file name
	imgName := filepath.Base(uri)
	if strings.Contains(imgName, "?") {
		imgName = imgName[:strings.Index(imgName, "?")]
	}
	imgName, err = url.QueryUnescape(imgName)
	if err != nil {
		return "", err
	}
	if c := resp.Header.Get("Content-Type"); strings.Contains(c, "image/") {
		ext := "." + strings.TrimPrefix(c, "image/")

		if filepath.Ext(imgName) == "" {
			imgName += ext
		}
	}

	// Check if the file already exists
	path := filepath.Join(dir, imgName)
	if _, err := os.Stat(path); err == nil {
		path = filepath.Join(dir, strconv.Itoa(rand.Int())+"-"+imgName)
	}

	// Save the image
	img, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer img.Close()

	_, err = io.Copy(img, resp.Body)
	if err != nil {
		return "", err
	}

	// Return everything after "/static" in path
	return path[strings.Index(path, "/static")+7:], nil
}

var delim = []byte("+++\n")

func writeEntry(e Entry, dir string) error {
	slug := makePath(e.Title)

	if len(e.Slug) > 0 && flags.SlugName {
		slug = e.Slug
	}

	filename := filepath.Join(dir, slug+".md")
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, e)
}

func writeComment(e Entry, dir string) error {
	e.Title = strings.Replace(strings.Replace(e.Title, "\n", "", -1), "\r", "", -1)

	folder := filepath.Join(dir, "comments")
	filename := filepath.Join(folder, "c"+e.ID+".toml")

	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		return err
	}

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return ct.Execute(f, e)
}

// Take a string with any characters and replace it so the string could be used in a filepath.
// E.g. Social Media -> social-media
func makePath(s string) string {
	return unicodeSanitize(strings.ToLower(strings.Replace(strings.TrimSpace(s), " ", "-", -1)))
}

func unicodeSanitize(s string) string {
	source := []rune(s)
	target := make([]rune, 0, len(source))

	for _, r := range source {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			target = append(target, r)
		}
	}

	return string(target)
}
