package main

import (
	"runtime/debug"
	"testing"
)

// fakeInfo — build info с заданной версией Main.
func fakeInfo(v string) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Version: v}}
}

func TestResolveVersion_NilBuildInfoIsDev(t *testing.T) {
	if got := resolveVersion(nil); got != "dev" {
		t.Errorf("resolveVersion(nil) = %q, ожидала dev", got)
	}
}

func TestResolveVersion_DevelIsDev(t *testing.T) {
	if got := resolveVersion(fakeInfo("(devel)")); got != "dev" {
		t.Errorf("resolveVersion((devel)) = %q, ожидала dev", got)
	}
}

func TestResolveVersion_EmptyIsDev(t *testing.T) {
	if got := resolveVersion(fakeInfo("")); got != "dev" {
		t.Errorf("resolveVersion(\"\") = %q, ожидала dev", got)
	}
}

func TestResolveVersion_TagStripsV(t *testing.T) {
	if got := resolveVersion(fakeInfo("v0.2.7")); got != "0.2.7" {
		t.Errorf("resolveVersion(v0.2.7) = %q, ожидала 0.2.7 (без v)", got)
	}
}

func TestVersionFlag_PrintsNameAndVersion(t *testing.T) {
	out := runGotimeline(t, "-version")
	if !contains(out, "gotimeline ") {
		t.Errorf("вывод -version должен начинаться с имени: %q", out)
	}
	if !contains(out, "dev") {
		t.Errorf("локальная сборка без тега: ожидала dev, получено %q", out)
	}
}
