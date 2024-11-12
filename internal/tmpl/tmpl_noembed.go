//go:build noembed

package tmpl

import "os"

func init() {
	templates = os.DirFS("./web")
}
