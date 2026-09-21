package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/schuettc/kempt/internal/jsonutil"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

func CheckDupIdentity(ctx *machine.Context, pkgs []*manifest.Package) []Finding {
	var out []Finding
	for _, pkg := range pkgs {
		for _, step := range pkg.Steps {
			jm, ok := step.(manifest.JSONMergeStep)
			if !ok {
				continue
			}
			file := ctx.Expand(jm.File)
			b, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			var live map[string]any
			if json.Unmarshal(b, &live) != nil {
				continue
			}
			for key := range jm.Merge {
				current, ok := live[key].([]any)
				if !ok {
					continue
				}
				byID := map[string][]string{}
				for _, e := range current {
					s, ok := e.(string)
					if !ok {
						continue
					}
					id := jsonutil.SpecIdentity(s)
					byID[id] = append(byID[id], s)
				}
				var ids []string
				for id, variants := range byID {
					if len(variants) > 1 {
						ids = append(ids, id)
					}
				}
				sort.Strings(ids)
				for _, id := range ids {
					out = append(out, Finding{
						Check:       "dup-identity",
						Package:     pkg.Name,
						Severity:    Warn,
						Detail:      fmt.Sprintf("%s .%s lists %q %d times: %v", file, key, id, len(byID[id]), byID[id]),
						Remediation: "run `kempt apply` (an arrays=replace merge collapses these) or dedupe the file",
					})
				}
			}
		}
	}
	return out
}
