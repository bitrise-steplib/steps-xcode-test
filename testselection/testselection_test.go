package testselection

import (
	"os"
	"path/filepath"
	"testing"
)

const selected = `{"version":1,"mode":"selected","reason":"2 changed files mapped onto the project graph","tests":[
 {"id":"spm:Core/CoreTests","name":"CoreTests","ecosystem":"spm","selected":false,"reason":"no changed dependency","identifiers":{"only_testing":"CoreTests"}},
 {"id":"xcode:App/AppTests","name":"AppTests","ecosystem":"xcode","selected":true,"reason":"via App","identifiers":{"only_testing":"AppTests"}},
 {"id":"gradle::core:unitTest","name":":core unit tests","ecosystem":"gradle","selected":true,"reason":"own file","identifiers":{"gradle_module":":core","gradle_task":":core:test"}}]}`

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test-selection.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadEmptyPathIsNoSelection(t *testing.T) {
	sel, err := Load("  ")
	if err != nil || sel != nil {
		t.Fatalf("got %v, %v", sel, err)
	}
	d, err := Decide(sel, OnEmptySkip, "xcode")
	if err != nil || d.Action != RunAll {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestDecideSelectedFiltersByEcosystem(t *testing.T) {
	sel, err := Load(write(t, selected))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decide(sel, OnEmptySkip, "xcode", "spm")
	if err != nil || d.Action != RunSelected || len(d.Tests) != 1 || d.Tests[0].Identifiers.OnlyTesting != "AppTests" {
		t.Fatalf("got %+v, %v", d, err)
	}
	d, err = Decide(sel, OnEmptySkip, "gradle")
	if err != nil || d.Action != RunSelected || d.Tests[0].Identifiers.GradleTask != ":core:test" {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestDecideEmptyPolicies(t *testing.T) {
	sel, err := Load(write(t, selected))
	if err != nil {
		t.Fatal(err)
	}
	if d, err := Decide(sel, OnEmptySkip, "flutter"); err != nil || d.Action != Skip {
		t.Fatalf("skip: got %+v, %v", d, err)
	}
	if d, err := Decide(sel, OnEmptyRunAll, "flutter"); err != nil || d.Action != RunAll {
		t.Fatalf("run_all: got %+v, %v", d, err)
	}
	if _, err := Decide(sel, OnEmptyFail, "flutter"); err == nil {
		t.Fatal("fail: expected an error")
	}
}

func TestDecideFullAndShadowRunEverything(t *testing.T) {
	for _, mode := range []string{"full", "shadow"} {
		sel, err := Load(write(t, `{"version":1,"mode":"`+mode+`","reason":"x","tests":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		if d, err := Decide(sel, OnEmptySkip, "xcode"); err != nil || d.Action != RunAll {
			t.Fatalf("%s: got %+v, %v", mode, d, err)
		}
	}
}

func TestDecideRejectsUnknownPolicy(t *testing.T) {
	if _, err := Decide(nil, "maybe", "xcode"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoadRejectsUnknownVersion(t *testing.T) {
	if _, err := Load(write(t, `{"version":2,"mode":"full"}`)); err == nil {
		t.Fatal("expected an error")
	}
}
