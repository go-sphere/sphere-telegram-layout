//go:build embed_dash

package dash

import "embed"

// IMPORTANT:
// All files in the subtree rooted at that directory are embedded (recursively), except that files with names beginning with ‘.’ or ‘_’ are excluded.

// Put the dashboard build output in $(DASH_DIR) (see Makefile) and run
// `make build/assets` to copy it into dashboard/dist.

//go:embed dashboard/dist
var Assets embed.FS

var AssetsPath = "dashboard/dist"
