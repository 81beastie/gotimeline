package main

import (
	"runtime/debug"
	"strings"
)

const devVersion = "dev"

// version — из build info (тега релиза); локальная сборка без тега = dev.
var version = resolveVersion(mustBuildInfo())

func mustBuildInfo() *debug.BuildInfo {
	info, ok := currentBuildInfo()
	if !ok {
		return nil
	}
	return info
}

func currentBuildInfo() (*debug.BuildInfo, bool) {
	return debug.ReadBuildInfo()
}

// resolveVersion — тег v0.2.7 → "0.2.7"; (devel)/пусто → dev.
func resolveVersion(info *debug.BuildInfo) string {
	if info == nil {
		return devVersion
	}
	if info.Main.Version == "" || info.Main.Version == "(devel)" {
		return devVersion
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}
