package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unicode"
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
	return fmt.Errorf("Unknown value for draft boolean: %s", v)
}

type Author struct {
	Name string `xml:"name"`
	Uri  string `xml:"uri"`
}

type Export struct {
	XMLName xml.Name `xml:"feed"`
	Entries []Entry  `xml:"entry"`
}

type Entry struct {
	ID        string `xml:"id"`
	Published Date   `xml:"published"`
	Updated   Date   `xml:"updated"`
	Draft     Draft  `xml:"control>draft"`
	Title     string `xml:"title"`
	Content   string `xml:"content"`
	Tags      Tags   `xml:"category"`
	Author    Author `xml:"author"`
	Extra     string
}
type Tag struct {
	Name   string `xml:"term,attr"`
	Scheme string `xml:"scheme,attr"`
}

type Tags []Tag

func (t Tags) TomlString() string {
	names := []string{}
	for _, t := range t {
		if t.Scheme == "http://www.blogger.com/atom/ns#" {
			names = append(names, fmt.Sprintf("%q", t.Name))
		}
	}
	return strings.Join(names, ", ")
}

var templ = `+++
title = "{{ .Title }}"
date = {{ .Published }}
updated = {{ .Updated }}{{ with .Tags.TomlString }}
tags = [{{ . }}]{{ end }}{{ if .Draft }}
draft = true{{ end }}
blogimport = true {{ with .Extra }}
{{.}}{{ end }}
[author]
	name = "{{ .Author.Name }}"
	uri = "{{ .Author.Uri }}"
+++

{{ .Content }}
`

var t = template.Must(template.New("").Parse(templ))

func main() {
	log.SetFlags(0)

	extra := flag.String("extra", "", "additional metadata to set in frontmatter")
	flag.Parse()

	args := flag.Args()

	if len(args) != 2 {
		log.Printf("Usage: %s [options] <xmlfile> <targetdir>", os.Args[0])
		log.Println("options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	dir := args[1]

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
	}
	if err != nil {
		log.Fatal(err)
	}

	if !info.IsDir() {
		log.Fatal("Second argument is not a directory.")
	}

	b, err := ioutil.ReadFile(args[0])
	if err != nil {
		log.Fatal(err)
	}

	exp := Export{}

	err = xml.Unmarshal(b, &exp)
	if err != nil {
		log.Fatal(err)
	}

	if len(exp.Entries) < 1 {
		log.Fatal("No blog entries found!")
	}

	count := 0
	drafts := 0
	for _, entry := range exp.Entries {
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
		if extra != nil {
			entry.Extra = *extra
		}
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

var delim = []byte("+++\n")

func writeEntry(e Entry, dir string) error {
	filename := filepath.Join(dir, makePath(e.Published, e.Title)+".md")
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, e)
}

// Take a string with any characters and replace it so the string could be used in a path.
// E.g. Social Media -> social-media
func makePath(d Date, s string) string {
	return fmt.Sprintf("%v-%s", d.String()[:10], unicodeSanitize(strings.ToLower(strings.Replace(strings.TrimSpace(s), " ", "-", -1))))
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
