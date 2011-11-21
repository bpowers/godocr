// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/build"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath" // use for file system paths
	"regexp"
	"runtime"
	"strings"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: goinstall [flags] importpath...")
	fmt.Fprintln(os.Stderr, "       goinstall [flags] -a")
	flag.PrintDefaults()
	os.Exit(2)
}

const logfile = "goinstall.log"

var (
	fset          = token.NewFileSet()
	argv0         = os.Args[0]
	errors_       = false
	parents       = make(map[string]string)
	visit         = make(map[string]status)
	installedPkgs = make(map[string]map[string]bool)
	schemeRe      = regexp.MustCompile(`^[a-z]+://`)

	verbose           = flag.Bool("v", false, "verbose")
)

type status int // status for visited map
const (
	unvisited status = iota
	visiting
	done
)

func logf(format string, args ...interface{}) {
	format = "%s: " + format
	args = append([]interface{}{argv0}, args...)
	fmt.Fprintf(os.Stderr, format, args...)
}

func printf(format string, args ...interface{}) {
	if *verbose {
		logf(format, args...)
	}
}

func errorf(format string, args ...interface{}) {
	errors_ = true
	logf(format, args...)
}

func terrorf(tree *build.Tree, format string, args ...interface{}) {
	if tree != nil && tree.Goroot && os.Getenv("GOPATH") == "" {
		format = strings.TrimRight(format, "\n") + " ($GOPATH not set)\n"
	}
	errorf(format, args...)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if runtime.GOROOT() == "" {
		fmt.Fprintf(os.Stderr, "%s: no $GOROOT\n", argv0)
		os.Exit(1)
	}
	readPackageList()

	// special case - "unsafe" is already installed
	visit["unsafe"] = done

	args := flag.Args()
	if len(args) == 0 {
		usage()
	}
	for _, path := range args {
		if s := schemeRe.FindString(path); s != "" {
			errorf("%q used in import path, try %q\n", s, path[len(s):])
			continue
		}
		document(path)
	}
	if errors_ {
		os.Exit(1)
	}
}

// printDeps prints the dependency path that leads to pkg.
func printDeps(pkg string) {
	if pkg == "" {
		return
	}
	if visit[pkg] != done {
		printDeps(parents[pkg])
	}
	fmt.Fprintf(os.Stderr, "\t%s ->\n", pkg)
}

// readPackageList reads the list of installed packages from the
// goinstall.log files in GOROOT and the GOPATHs and initalizes
// the installedPkgs variable.
func readPackageList() {
	for _, t := range build.Path {
		installedPkgs[t.Path] = make(map[string]bool)
		name := filepath.Join(t.Path, logfile)
		pkglistdata, err := ioutil.ReadFile(name)
		if err != nil {
			printf("%s\n", err)
			continue
		}
		pkglist := strings.Fields(string(pkglistdata))
		for _, pkg := range pkglist {
			installedPkgs[t.Path][pkg] = true
		}
	}
}

// logPackage logs the named package as installed in the goinstall.log file
// in the given tree if the package is not already in that file.
func logPackage(pkg string, tree *build.Tree) (logged bool) {
	if installedPkgs[tree.Path][pkg] {
		return false
	}
	name := filepath.Join(tree.Path, logfile)
	fout, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		terrorf(tree, "package log: %s\n", err)
		return false
	}
	fmt.Fprintf(fout, "%s\n", pkg)
	fout.Close()
	return true
}

// show godoc of current package
func document(pkg string) {
	// Don't allow trailing '/'
	if strings.HasSuffix(pkg, "/") {
		errorf("%s should not have trailing '/'\n", pkg)
		return
	}

	// Check whether package is local or remote.
	// If remote, download or update it.
	tree, pkg, err := build.FindTree(pkg)

	var dir, baseDir string
	// Download remote packages if not found or forced with -u flag.
	remote := isRemote(pkg)
	if !remote {
		dir = filepath.Join(tree.SrcDir(), filepath.FromSlash(pkg))
	} else {
		baseDir, _ = ioutil.TempDir("", "godocr")
		_, err = download(pkg, baseDir)
		if err != nil {
			errorf("%s: problem downloading: %s\n", pkg, err)
			return
		}
		dir = filepath.Join(baseDir, filepath.FromSlash(pkg))
	}

	cmd := exec.Command("godoc", ".")
	cmd.Stdin = bytes.NewBuffer(nil)
	cmd.Dir = dir
	printf("%s: %s %s\n", dir, cmd.Path, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		errorf("%s: godoc: %s\n", pkg, err)
	} else {
		fmt.Print(string(out))
	}

	// only clean up after ourselves if we've downloaded a remote
	// directory
	if remote {
		err = os.RemoveAll(baseDir)
		if err != nil {
			errorf("%s: couldn't clean up after ourselves: %s\n", pkg, err)
		}
	}
}

// Is this a standard package path?  strings container/list etc.
// Assume that if the first element has a dot, it's a domain name
// and is not the standard package path.
func isStandardPath(s string) bool {
	dot := strings.Index(s, ".")
	slash := strings.Index(s, "/")
	return dot < 0 || 0 < slash && slash < dot
}

// run runs the command cmd in directory dir with standard input stdin.
// If the command fails, run prints the command and output on standard error
// in addition to returning a non-nil error.
func run(dir string, stdin []byte, cmd ...string) error {
	return genRun(dir, stdin, cmd, false)
}

// quietRun is like run but prints nothing on failure unless -v is used.
func quietRun(dir string, stdin []byte, cmd ...string) error {
	return genRun(dir, stdin, cmd, true)
}

// genRun implements run and quietRun.
func genRun(dir string, stdin []byte, arg []string, quiet bool) error {
	cmd := exec.Command(arg[0], arg[1:]...)
	cmd.Stdin = bytes.NewBuffer(stdin)
	cmd.Dir = dir
	printf("%s: %s %s\n", dir, cmd.Path, strings.Join(arg[1:], " "))
	out, err := cmd.CombinedOutput()
	if err != nil {
		if !quiet || *verbose {
			if dir != "" {
				dir = "cd " + dir + "; "
			}
			fmt.Fprintf(os.Stderr, "%s: === %s%s\n", cmd.Path, dir, strings.Join(cmd.Args, " "))
			os.Stderr.Write(out)
			fmt.Fprintf(os.Stderr, "--- %s\n", err)
		}
		return errors.New("running " + arg[0] + ": " + err.Error())
	}
	return nil
}
