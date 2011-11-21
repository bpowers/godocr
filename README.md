godocr - godoc for remote repositories
======================================

A super simple utility for viewing the godocs of a remote repository
in a terminal.  godocr clones a remote repository into a temporary
directory, runs:

    $ cd $TEMPDIR; godoc .

and then removes the temporary directory.  There are a number of ways
to make this better, such as smartly fetching gzipped snapshots of the
latest commit from github (as a full checkout of a large project is a
fairly heavyweight operation), and enabling the pretty-printed http
server.

Possibly of interest is that if godocr is run on a package that is
already installed, or in the standard library, it simply calls godoc.


license
-------

licensed the same as Go, as its simply a derivative of goinstall. 

