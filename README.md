# 📚 Blogimport
[![Go Report Card](https://goreportcard.com/badge/github.com/natefinch/blogimport)](https://goreportcard.com/report/github.com/natefinch/blogimport)
[![Apache V2 License](https://img.shields.io/badge/license-MIT-blue)](https://opensource.org/licenses/MIT)

>Blogimport is a simple tool to convert Blogger's xml export data into
[Hugo](http://hugo.spf13.com)-friendly toml/markdown format.

## Description
The tool is really basic, you just pass it the name of the file you're
converting, the directory in which to output the markdown files

Blogimport outputs the tags, the post title, the published date, and whether or
not it's a draft as standard Hugo frontmatter. In addition, the updated date is
added, as well as author name and uri, and an additional value of blogimport =
true (which can be handy for having special handling in your Hugo templates for
imported posts).

The original HTML content can be converted into Markdown markup and is ouput as the main content of the TOML file. Note, by default that no processing is done on the content... HTML is valid markdown.

Here's a typical frontmatter output:
``` toml
+++
title = "Wubba lubba dub dub!"
date = 2014-07-09T17:43:00Z
updated = 2014-07-22T07:11:52Z
tags = ["Hugo", "is", "awesome"]
draft = true
blogimport = true 
[author]
	name = "Nate Finch"
	uri = "https://plus.google.com/115818189328363361527"
+++

'Quantum carburetor'? Jesus, Morty. You can't just add a Sci-Fi word to a car word and hope it means something... Huh, looks like something's wrong with the microverse battery. We're gonna have to go inside. Burgertime! You want to see my first boner, or should we go straight to the moment I discovered interdimensional travel? Oh, it gets darker, Morty. Welcome to the darkest year of our adventures. First thing that's different? No more dad, Morty. 
```

Note that only toml output is supported now.  If you want to support something
else, feel free to make a pull request.  I set up the code to be pretty easy to
update to output other formats.

## Install
With the official Go tool
```bash
$ go install github.com/natefinch/blogimport@latest
```

## Usage
```bash
Usage: blogimport [options] <xmlfile> <targetdir>
options:
  -comments
    	import comments
  -extra string
    	additional metadata to set in frontmatter
  -md
    	convert HTML to markdown
  -no-blogger
    	remove blogger specific url
  -no-slug
    	-no-slug=false for not use slug as file name (default true)
  -static string
    	static directory for import images
```