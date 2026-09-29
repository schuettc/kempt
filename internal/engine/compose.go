package engine

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/schuettc/kempt/internal/jsonutil"
	"github.com/schuettc/kempt/internal/machine"
	"github.com/schuettc/kempt/internal/manifest"
)

// composed is the selection after layer composition, plus what the plan should
// say about it: the contributors and overrides of each folded merge step, keyed
// by package index then step index.
type composed struct {
	pkgs      []*manifest.Package
	notes     map[int]map[int]string
	overrides map[int][]string // package index -> pin overrides it made
}

// layerLabel names a package's layer in plan output.
func layerLabel(p *manifest.Package) string {
	if p.Layer == "" {
		return "base"
	}
	return p.Layer
}

// compose applies the cross-layer rules of the layers spec to the selected
// packages, in order: a later layer's pin of a pi/npm entry replaces an
// earlier layer's; two layers claiming one exclusive target is an error unless
// the definitions are identical (one copy is kept); json-merge and toml-merge
// steps into one file from two or more layers fold into one step. Packages from
// a single layer are returned unchanged, so a base-only selection plans
// exactly as it did before layers. The input packages are never mutated.
func compose(ctx *machine.Context, in []*manifest.Package) (*composed, error) {
	pkgs := make([]*manifest.Package, len(in))
	for i, p := range in {
		c := *p
		c.Steps = append([]manifest.Step(nil), p.Steps...)
		pkgs[i] = &c
	}
	c := &composed{pkgs: pkgs, notes: map[int]map[int]string{}, overrides: map[int][]string{}}
	if !multiLayer(pkgs) {
		return c, nil
	}
	c.overridePins()
	if err := resolveExclusive(ctx, pkgs); err != nil {
		return nil, err
	}
	c.foldMerges(ctx)
	return c, nil
}

func multiLayer(pkgs []*manifest.Package) bool {
	for _, p := range pkgs {
		if p.Layer != pkgs[0].Layer {
			return true
		}
	}
	return false
}

// active reports whether step s of package p applies on this machine: a step
// its only-clause skips takes no part in composition and renders as skipped.
func active(ctx *machine.Context, p *manifest.Package, s manifest.Step) bool {
	if _, skip := skipReason(ctx, p.Only); skip {
		return false
	}
	_, skip := skipReason(ctx, stepOnly(s))
	return !skip
}

// ---- pins ------------------------------------------------------------------

type pinRef struct {
	pkg, step int
	pi        bool
	entry     string
}

// overridePins drops an earlier layer's pi/npm install entry when a later layer
// lists the same package (by SpecIdentity) with a different spec, and notes the
// override on the later package.
func (c *composed) overridePins() {
	pkgs := c.pkgs
	seen := map[string]pinRef{}
	for pi, p := range pkgs {
		for si, s := range p.Steps {
			is, ok := s.(manifest.InstallStep)
			if !ok {
				continue
			}
			visit := func(entries []string, isPi bool) {
				for _, e := range entries {
					key := fmt.Sprintf("%v|%s", isPi, jsonutil.SpecIdentity(e))
					prev, ok := seen[key]
					if ok && pkgs[prev.pkg].Layer != p.Layer && prev.entry != e {
						dropEntry(pkgs[prev.pkg], prev.step, prev.pi, prev.entry)
						c.overrides[pi] = append(c.overrides[pi], fmt.Sprintf("%s (%s overrides %s: %s)", e, layerLabel(p), layerLabel(pkgs[prev.pkg]), prev.entry))
					}
					seen[key] = pinRef{pi, si, isPi, e}
				}
			}
			visit(is.Pi, true)
			visit(is.Npm, false)
		}
	}
}

func dropEntry(p *manifest.Package, step int, pi bool, entry string) {
	is := p.Steps[step].(manifest.InstallStep)
	list := &is.Npm
	if pi {
		list = &is.Pi
	}
	var kept []string
	for _, e := range *list {
		if e != entry {
			kept = append(kept, e)
		}
	}
	*list = kept
	p.Steps[step] = is
}

// ---- exclusive targets -----------------------------------------------------

// exclusiveKey names the one thing a step owns outright, and a comparable form
// of the step for the identical-definition check; ok is false for steps that
// own nothing exclusively.
func exclusiveKey(ctx *machine.Context, s manifest.Step) (key string, norm any, ok bool) {
	switch st := s.(type) {
	case manifest.SymlinkStep:
		// The source resolves against the declaring layer's root, so two layers
		// naming the same relative from are different links.
		st.From = ctx.Expand(st.From)
		return "symlink " + st.To, st, true
	case manifest.GitCloneStep:
		return "git-clone " + st.To, st, true
	case manifest.DownloadStep:
		bin := st.Bin
		if bin == "" {
			bin = st.Tool
		}
		return "binary " + bin, st, true
	case manifest.GithubReleaseStep:
		return "binary " + st.Bin, st, true
	case manifest.ServiceStep:
		return "service " + st.Label, st, true
	}
	return "", nil, false
}

type claim struct {
	pkg  int
	norm any
}

// resolveExclusive rejects two layers claiming one exclusive target with
// different definitions, and drops the later copy of an identical one. Claims
// within one layer are left alone, as before layers.
func resolveExclusive(ctx *machine.Context, pkgs []*manifest.Package) error {
	claims := map[string]claim{}
	for pi, p := range pkgs {
		pctx := ContextFor(ctx, p.Root)
		var kept []manifest.Step
		for _, s := range p.Steps {
			key, norm, ok := exclusiveKey(pctx, s)
			if !ok || !active(ctx, p, s) {
				kept = append(kept, s)
				continue
			}
			prev, seen := claims[key]
			if !seen || pkgs[prev.pkg].Layer == p.Layer {
				claims[key] = claim{pi, norm}
				kept = append(kept, s)
				continue
			}
			if !reflect.DeepEqual(prev.norm, norm) {
				q := pkgs[prev.pkg]
				return fmt.Errorf("conflict: %s is declared by %s (%s) and %s (%s)", key, layerLabel(q), q.Name, layerLabel(p), p.Name)
			}
		}
		p.Steps = kept
	}
	return nil
}

// ---- merge folding ---------------------------------------------------------

type mergeRef struct{ pkg, step int }

type mergeGroup struct {
	kind, file, mode string
	refs             []mergeRef
}

// foldMerges folds every json-merge (per arrays mode) and toml-merge into one
// file, when the contributors span two or more layers, into a single step at
// the first contributor's position. When a file has both an append and a
// replace fold, the replace fold is moved after the last append contributor so
// replace wins on a shared key.
func (c *composed) foldMerges(ctx *machine.Context) {
	pkgs := c.pkgs
	groups := map[string]*mergeGroup{}
	var order []string
	for pi, p := range pkgs {
		pctx := ContextFor(ctx, p.Root)
		for si, s := range p.Steps {
			var kind, file, mode string
			switch st := s.(type) {
			case manifest.JSONMergeStep:
				kind, file, mode = "json-merge", pctx.Expand(st.File), st.Arrays
				if mode == "" {
					mode = "append"
				}
			case manifest.TomlMergeStep:
				kind, file = "toml-merge", pctx.Expand(st.File)
			default:
				continue
			}
			if !active(ctx, p, s) {
				continue
			}
			key := kind + "|" + file + "|" + mode
			g, ok := groups[key]
			if !ok {
				g = &mergeGroup{kind: kind, file: file, mode: mode}
				groups[key] = g
				order = append(order, key)
			}
			g.refs = append(g.refs, mergeRef{pi, si})
		}
	}

	remove := map[mergeRef]bool{}
	replaceAt := map[string]mergeRef{} // file -> the replace fold's position
	lastAppend := map[string]mergeRef{}
	for _, key := range order {
		g := groups[key]
		if g.mode == "append" && g.kind == "json-merge" {
			lastAppend[g.file] = g.refs[len(g.refs)-1]
		}
		layers := map[string]bool{}
		for _, r := range g.refs {
			layers[pkgs[r.pkg].Layer] = true
		}
		if len(layers) < 2 {
			continue
		}
		merged, contributors, overrides := foldGroup(ctx, pkgs, g)
		first := g.refs[0]
		switch g.kind {
		case "json-merge":
			arrays := g.mode
			if arrays == "append" {
				arrays = ""
			}
			pkgs[first.pkg].Steps[first.step] = manifest.JSONMergeStep{File: g.file, Merge: merged, Arrays: arrays}
			if g.mode == "replace" {
				replaceAt[g.file] = first
			}
		case "toml-merge":
			pkgs[first.pkg].Steps[first.step] = manifest.TomlMergeStep{File: g.file, Merge: merged}
		}
		note := "(from " + strings.Join(contributors, ", ") + ")"
		if len(overrides) > 0 {
			note += "; " + strings.Join(overrides, "; ")
		}
		c.setNote(first, note)
		for _, r := range g.refs[1:] {
			remove[r] = true
		}
	}

	// Move a replace fold after the file's last append contributor.
	move := map[mergeRef]mergeRef{} // replace fold -> insert after
	for file, at := range replaceAt {
		if after, ok := lastAppend[file]; ok && less(at, after) {
			move[at] = after
		}
	}
	c.rebuild(remove, move)
}

func less(a, b mergeRef) bool {
	return a.pkg < b.pkg || (a.pkg == b.pkg && a.step < b.step)
}

func (c *composed) setNote(r mergeRef, note string) {
	if c.notes[r.pkg] == nil {
		c.notes[r.pkg] = map[int]string{}
	}
	c.notes[r.pkg][r.step] = note
}

// rebuild drops the folded-away steps and moves each replace fold after its
// append anchor, re-keying the notes to the steps' final positions.
func (c *composed) rebuild(remove map[mergeRef]bool, move map[mergeRef]mergeRef) {
	type moved struct {
		step manifest.Step
		note string
	}
	pending := map[mergeRef][]moved{}
	for from, after := range move {
		pending[after] = append(pending[after], moved{c.pkgs[from.pkg].Steps[from.step], c.notes[from.pkg][from.step]})
		remove[from] = true
	}
	notes := map[int]map[int]string{}
	for pi, p := range c.pkgs {
		var steps []manifest.Step
		add := func(s manifest.Step, note string) {
			if note != "" {
				if notes[pi] == nil {
					notes[pi] = map[int]string{}
				}
				notes[pi][len(steps)] = note
			}
			steps = append(steps, s)
		}
		for si, s := range p.Steps {
			r := mergeRef{pi, si}
			if !remove[r] {
				add(s, c.notes[pi][si])
			}
			for _, m := range pending[r] {
				add(m.step, m.note)
			}
		}
		p.Steps = steps
	}
	c.notes = notes
}

// foldGroup combines a group's merge documents in order: maps deep-merge,
// arrays take the ordered union, and a scalar set differently by a later
// layer wins and is reported as an override.
func foldGroup(ctx *machine.Context, pkgs []*manifest.Package, g *mergeGroup) (map[string]any, []string, []string) {
	var acc any = map[string]any{}
	setBy := map[string]string{}
	var contributors, overrides []string
	seen := map[string]bool{}
	for _, r := range g.refs {
		p := pkgs[r.pkg]
		label := layerLabel(p)
		if !seen[label] {
			seen[label] = true
			contributors = append(contributors, label)
		}
		var doc map[string]any
		switch st := p.Steps[r.step].(type) {
		case manifest.JSONMergeStep:
			doc = st.Merge
		case manifest.TomlMergeStep:
			doc = st.Merge
		}
		acc = combine(acc, jsonutil.ToAny(doc), "", label, setBy, &overrides)
	}
	sort.Strings(overrides)
	out, _ := acc.(map[string]any)
	return out, contributors, overrides
}

func combine(acc, next any, path, layer string, setBy map[string]string, overrides *[]string) any {
	switch n := next.(type) {
	case map[string]any:
		a, ok := acc.(map[string]any)
		if !ok {
			a = map[string]any{}
		}
		out := map[string]any{}
		for k, v := range a {
			out[k] = v
		}
		for k, v := range n {
			sub := k
			if path != "" {
				sub = path + "." + k
			}
			if cur, ok := out[k]; ok {
				out[k] = combine(cur, v, sub, layer, setBy, overrides)
			} else {
				out[k] = v
				setBy[sub] = layer
			}
		}
		return out
	case []any:
		a, _ := acc.([]any)
		out := append([]any(nil), a...)
		for _, e := range n {
			dup := false
			for _, have := range out {
				if reflect.DeepEqual(have, e) {
					dup = true
					break
				}
			}
			if !dup {
				out = append(out, e)
			}
		}
		setBy[path] = layer
		return out
	default:
		if acc != nil && !reflect.DeepEqual(acc, next) && setBy[path] != layer {
			*overrides = append(*overrides, fmt.Sprintf("%s: %s overrides %s", path, layer, setBy[path]))
		}
		setBy[path] = layer
		return next
	}
}
