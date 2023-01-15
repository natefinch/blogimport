# Project Title
Blogimport

## Description
Blogimport is a simple tool to convert Blogger's xml export data into
[Hugo](http://hugo.spf13.com)-friendly markdown format.

The tool is really basic, you just pass it the name of the file you're converting, the directory in which to output the markdown files, and optionally a piece of metadata to add to each of the posts' frontmatter (which must be a single line of valid toml, such as -extra="type =  "oldPost\"").

Blogimport outputs the tags, the post title, the published date, and whether or not it's a draft as standard Hugo frontmatter.  In addition, the updated date is added, as well as author name and uri, and an additional value of blogimport = true (which can be handy for having special handling in your Hugo templates for imported posts).

Finally, the original HTML content is ouput as the main content of the markdown file.  Note that no processing is done on the content.. HTML is valid markdown, and it's probably best not to muck with it, so blogimport doesn't touch it. 

Note that it now supports toml and yaml, but by default it will now use yaml.  If you want to support something else, feel free to make a pull request.  I set up the code to be pretty easy to update to output other formats.

## Getting Started

### Dependencies

The latest version of Go should be installed from the [Go Install page](https://go.dev/doc/install)

### Executing program
The program is run from the command line and requires two arguments: 	 

- xmlfile - The Blogger xml export file
- targetdir - The directory to output the new Hugo formatted files

#### Step-by-step
1. Open up your command line tool (Terminal on macOS)
1. Navigate to the repository directory
1. Run `go run . <xmlfile> <targetdir>`

Here's a typical frontmatter output:

	---
	title: "My cool title"
	date: 2014-07-09T17:43:00Z
	updated: 2014-07-22T07:11:52Z
	tags: ["Hugo", "is", "awesome"]
	draft: true
	blogimport:  true 
	author: "Nate Finch"
	---


## License

This project is licensed under the MIT License - see the LICENSE.md file for details

