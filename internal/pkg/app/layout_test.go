package app

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

type layoutManifest struct {
	SchemaVersion int `json:"schema_version"`
	Ownership     struct {
		Generated   []string `json:"generated"`
		LayoutOwned []string `json:"layout_owned"`
		Mixed       []string `json:"mixed"`
		Default     string   `json:"default"`
	} `json:"ownership"`
}

func TestLayoutOwnershipHasNoOverlaps(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, ".sphere", "layout.json"))
	if err != nil {
		t.Fatalf("read .sphere/layout.json: %v", err)
	}
	var manifest layoutManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode .sphere/layout.json: %v", err)
	}
	if manifest.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", manifest.SchemaVersion)
	}
	if manifest.Ownership.Default != "project_owned" {
		t.Fatalf("ownership.default = %q, want project_owned", manifest.Ownership.Default)
	}

	type entry struct {
		bucket  string
		pattern string
	}
	var all []entry
	for _, p := range manifest.Ownership.Generated {
		all = append(all, entry{"generated", p})
	}
	for _, p := range manifest.Ownership.LayoutOwned {
		all = append(all, entry{"layout_owned", p})
	}
	for _, p := range manifest.Ownership.Mixed {
		all = append(all, entry{"mixed", p})
	}

	for i := range all {
		if all[i].pattern == "" {
			t.Fatalf("empty pattern in bucket %s", all[i].bucket)
		}
		for j := i + 1; j < len(all); j++ {
			if patternsOverlap(all[i].pattern, all[j].pattern) {
				t.Errorf("overlapping ownership patterns: (%s) %q and (%s) %q",
					all[i].bucket, all[i].pattern, all[j].bucket, all[j].pattern)
			}
		}
	}

	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		var matches []entry
		for _, e := range all {
			if matchLayoutPattern(e.pattern, rel) {
				matches = append(matches, e)
			}
		}
		if len(matches) > 1 {
			t.Errorf("file %q matched multiple ownership patterns: %+v", rel, matches)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
}

func patternsOverlap(a, b string) bool {
	return segsOverlap(strings.Split(a, "/"), strings.Split(b, "/"))
}

func segsOverlap(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) > 0 && a[0] == "**" {
		if len(a) == 1 {
			return len(b) > 0
		}
		for i := 0; i <= len(b); i++ {
			if segsOverlap(a[1:], b[i:]) {
				return true
			}
		}
		return false
	}
	if len(b) > 0 && b[0] == "**" {
		if len(b) == 1 {
			return len(a) > 0
		}
		for i := 0; i <= len(a); i++ {
			if segsOverlap(a[i:], b[1:]) {
				return true
			}
		}
		return false
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return singleSegOverlap(a[0], b[0]) && segsOverlap(a[1:], b[1:])
}

func singleSegOverlap(a, b string) bool {
	memo := make(map[[2]int]bool)
	var match func(i, j int) bool
	match = func(i, j int) bool {
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		var ok bool
		switch {
		case i == len(a) && j == len(b):
			ok = true
		case i < len(a) && a[i] == '*':
			ok = match(i+1, j) || (j < len(b) && match(i, j+1))
		case j < len(b) && b[j] == '*':
			ok = match(i, j+1) || (i < len(a) && match(i+1, j))
		case i < len(a) && j < len(b) && a[i] == b[j]:
			ok = match(i+1, j+1)
		}
		memo[key] = ok
		return ok
	}
	return match(0, 0)
}

func matchLayoutPattern(pattern, target string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		return strings.HasPrefix(target, prefix+"/")
	}
	ok, err := path.Match(pattern, target)
	return err == nil && ok
}
