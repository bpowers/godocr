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

godocr will simply call godoc for packages in the standard library,
but will check out a temporary copy of the latest version of a package
even if you have it installed.  This is potentially useful if you have
an older version goinstalled, but are interested in the godocs for the
latest development copy.


license
-------

licensed the same as Go, as its simply a derivative of goinstall. 

