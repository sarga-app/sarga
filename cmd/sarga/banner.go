package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

const bannerArt = `
   ___  __ _ _ __ __ _  __ _
  / __|/ _` + "`" + ` | '__/ _` + "`" + ` |/ _` + "`" + ` |
  \__ \ (_| | | | (_| | (_| |
  |___/\__,_|_|  \__, |\__,_|
                 |___/
`

// printBanner writes the ASCII banner + version to w only when w is a TTY.
// Suppressed for non-TTY (docker logs / journald / JSON consumers).
func printBanner(w io.Writer, version, commit string, dirty bool) {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return
	}
	dirtyMark := ""
	if dirty {
		dirtyMark = "-dirty"
	}
	fmt.Fprint(w, bannerArt)                                       //nolint:errcheck // banner is TTY-only, discard
	fmt.Fprintf(w, "  %s  (%s%s)\n\n", version, commit, dirtyMark) //nolint:errcheck // banner is TTY-only, discard
}
