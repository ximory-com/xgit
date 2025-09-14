package main

// main.go: minimal file for git.diff + gofmt preflight
// now with a tiny change to test modify hunks

// Hello returns a short greeting.
func Hello() string {
	return "hello, gitdiff"
}

// Add is a trivial function to create a second hunk and trigger gofmt.
func Add(a, b int) int {
	return a + b
}

// keep a trailing newline to satisfy both git and gofmt
