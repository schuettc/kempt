package manifest

import (
	"github.com/BurntSushi/toml"
)

type rawManifest struct {
	Kempt struct {
		Spec int `toml:"spec"`
	} `toml:"kempt"`
	Packages map[string]rawPackage `toml:"packages"`
	Profiles map[string]rawProfile `toml:"profiles"`
}

type rawProfile struct {
	Description string   `toml:"description"`
	Packages    []string `toml:"packages"`
}

type rawPackage struct {
	Description   string              `toml:"description"`
	Needs         []string            `toml:"needs"`
	Only          *Only               `toml:"only"`
	Install       []InstallStep       `toml:"install"`
	GithubRelease []GithubReleaseStep `toml:"github-release"`
	GitClone      []GitCloneStep      `toml:"git-clone"`
	Service       []ServiceStep       `toml:"service"`
	Symlink       []SymlinkStep       `toml:"symlink"`
	JSONMerge     []JSONMergeStep     `toml:"json-merge"`
	LineInFile    []LineInFileStep    `toml:"line-in-file"`
	Verify        []VerifyStep        `toml:"verify"`
}

// isFreeform reports whether an undecoded key falls within the free-form
// `merge` subtree of a json-merge step (segments `json-merge` then `merge`).
func isFreeform(key toml.Key) bool {
	for i := 0; i+1 < len(key); i++ {
		if key[i] == "json-merge" && key[i+1] == "merge" {
			return true
		}
	}
	return false
}

func Parse(src []byte) (*Manifest, []Finding) {
	var raw rawManifest
	md, err := toml.Decode(string(src), &raw)
	if err != nil {
		return nil, []Finding{{Path: "kempt.toml", Msg: err.Error()}}
	}
	var findings []Finding
	for _, key := range md.Undecoded() {
		if isFreeform(key) {
			// The `merge` field of a json-merge step is intentionally
			// free-form (map[string]any). BurntSushi reports its nested
			// keys as undecoded, so they must not be treated as unknown.
			continue
		}
		findings = append(findings, Finding{Path: key.String(), Msg: "unknown key"})
	}
	m := &Manifest{
		Spec:     raw.Kempt.Spec,
		Packages: map[string]*Package{},
		Profiles: map[string]*Profile{},
	}
	for name, rp := range raw.Packages {
		m.Packages[name] = &Package{
			Name:        name,
			Description: rp.Description,
			Needs:       rp.Needs,
			Only:        rp.Only,
			Steps:       interleave(md, name, rp),
		}
	}
	for name, pr := range raw.Profiles {
		m.Profiles[name] = &Profile{Name: name, Description: pr.Description, Packages: pr.Packages}
	}
	return m, findings
}

// interleave produces the ordered step slice for a package.
//
// NAIVE IMPLEMENTATION (Task 2): steps are appended in a fixed kind order
// (install, github-release, git-clone, service, symlink, json-merge,
// line-in-file, verify) rather than the order they appear in the manifest.
// Document-order interleaving is Task 3's job.
func interleave(_ toml.MetaData, _ string, rp rawPackage) []Step {
	var steps []Step
	for _, s := range rp.Install {
		steps = append(steps, s)
	}
	for _, s := range rp.GithubRelease {
		steps = append(steps, s)
	}
	for _, s := range rp.GitClone {
		steps = append(steps, s)
	}
	for _, s := range rp.Service {
		steps = append(steps, s)
	}
	for _, s := range rp.Symlink {
		steps = append(steps, s)
	}
	for _, s := range rp.JSONMerge {
		steps = append(steps, s)
	}
	for _, s := range rp.LineInFile {
		steps = append(steps, s)
	}
	for _, s := range rp.Verify {
		steps = append(steps, s)
	}
	return steps
}
