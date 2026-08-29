package picker

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	down  = tea.KeyMsg{Type: tea.KeyDown}
	up    = tea.KeyMsg{Type: tea.KeyUp}
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	space = tea.KeyMsg{Type: tea.KeySpace}
)

func key(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestProfilePreselectsPackages(t *testing.T) {
	m := newModel(
		[]Profile{{Name: "dev", Packages: []string{"core", "terminal"}}, {Name: "min", Packages: []string{"core"}}},
		[]Item{{Name: "core"}, {Name: "terminal"}, {Name: "nvim"}},
	)
	m2, _ := m.Update(enter)
	if !m2.(model).isChecked("core") || !m2.(model).isChecked("terminal") || m2.(model).isChecked("nvim") {
		t.Fatalf("profile dev should preselect core+terminal only")
	}
	if m2.(model).result().Confirmed {
		t.Fatal("advancing to package stage must not confirm")
	}
}

func TestToggleAndConfirm(t *testing.T) {
	m := newModel([]Profile{{Name: "min", Packages: []string{"core"}}}, []Item{{Name: "core"}, {Name: "nvim"}})
	m2, _ := m.Update(enter)  // choose "min" → core checked
	m3, _ := m2.Update(down)  // cursor → nvim
	m4, _ := m3.Update(space) // check nvim
	m5, _ := m4.Update(enter) // confirm
	r := m5.(model).result()
	if !r.Confirmed || len(r.Packages) != 2 {
		t.Fatalf("want confirmed core+nvim, got %+v", r)
	}
	if r.Packages[0] != "core" || r.Packages[1] != "nvim" {
		t.Fatalf("packages should be sorted, got %+v", r.Packages)
	}
	if r.Profile != "min" {
		t.Fatalf("want profile min, got %q", r.Profile)
	}
}

func TestQuitIsUnconfirmed(t *testing.T) {
	m := newModel([]Profile{{Name: "min"}}, []Item{{Name: "core"}})
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if m2.(model).result().Confirmed {
		t.Fatal("ctrl+c must not confirm")
	}
}

func TestQuitVariants(t *testing.T) {
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		key('q'),
	} {
		m := newModel([]Profile{{Name: "min"}}, []Item{{Name: "core"}})
		m2, _ := m.Update(msg)
		if m2.(model).result().Confirmed {
			t.Fatalf("%v must not confirm", msg)
		}
	}
}

func TestNavigationBounds(t *testing.T) {
	m := newModel(
		[]Profile{{Name: "a"}, {Name: "b"}},
		[]Item{{Name: "core"}, {Name: "nvim"}},
	)
	// up at top stays at 0
	m2, _ := m.Update(up)
	if m2.(model).cursor != 0 {
		t.Fatalf("up at top should stay at 0, got %d", m2.(model).cursor)
	}
	// down past bottom clamps
	m3, _ := m2.Update(down)
	m4, _ := m3.Update(down)
	m5, _ := m4.Update(down)
	if m5.(model).cursor != 1 {
		t.Fatalf("cursor should clamp to last profile index 1, got %d", m5.(model).cursor)
	}
}

func TestJKNavigation(t *testing.T) {
	m := newModel([]Profile{{Name: "min"}}, []Item{{Name: "core"}, {Name: "nvim"}, {Name: "tmux"}})
	m2, _ := m.Update(enter) // enter package stage
	m3, _ := m2.Update(key('j'))
	if m3.(model).cursor != 1 {
		t.Fatalf("j should move down, got %d", m3.(model).cursor)
	}
	m4, _ := m3.Update(key('k'))
	if m4.(model).cursor != 0 {
		t.Fatalf("k should move up, got %d", m4.(model).cursor)
	}
}

func TestSpaceInProfileModeIsNoop(t *testing.T) {
	m := newModel([]Profile{{Name: "dev", Packages: []string{"core"}}}, []Item{{Name: "core"}, {Name: "nvim"}})
	m2, _ := m.Update(space)
	if m2.(model).isChecked("core") || m2.(model).isChecked("nvim") {
		t.Fatal("space in profile mode must not toggle any item")
	}
}

func TestNoProfilesStartsInPackageStage(t *testing.T) {
	m := newModel(nil, []Item{{Name: "core"}, {Name: "nvim"}})
	// space toggles immediately (already in package stage)
	m2, _ := m.Update(space)
	if !m2.(model).isChecked("core") {
		t.Fatal("with no profiles, should start in package stage")
	}
	m3, _ := m2.Update(enter)
	r := m3.(model).result()
	if !r.Confirmed || len(r.Packages) != 1 || r.Packages[0] != "core" {
		t.Fatalf("want confirmed core, got %+v", r)
	}
}

func TestItemSelectedPreset(t *testing.T) {
	m := newModel(nil, []Item{{Name: "core", Selected: true}, {Name: "nvim"}})
	if !m.isChecked("core") || m.isChecked("nvim") {
		t.Fatal("preset Selected items should be checked initially")
	}
}
