package internal

import (
	"fmt"
	"net/http"
	"os"

	"pkg.jsn.cam/jsn/licenses"

	"go4.org/legal"
)

func init() {
	legal.RegisterLicense(licenses.MitLicense)
	legal.RegisterLicense(licenses.SQLiteBlessing)

	http.HandleFunc("/.jsn/licenses", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Licenses for this program: %s\n", os.Args[0])

		for _, li := range legal.Licenses() {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "---")
			fmt.Fprintln(w)
			fmt.Fprintln(w, li)
		}

		fmt.Fprintln(w)
		fmt.Fprintln(w, "---")
		fmt.Fprintln(w)

		fmt.Fprintln(w, "Be well, Creator.")
	})
}
