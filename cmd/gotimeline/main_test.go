package main

import (
	"io"
	"strings"
	"testing"
)

// runGotimeline — запуск main с аргументами; возвращает stdout.
func runGotimeline(t *testing.T, args ...string) string {
	t.Helper()
	old := osStdout
	r, w := io.Pipe()
	osStdout = w
	t.Cleanup(func() { osStdout = old })

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	exit := mainWithArgs(args)
	w.Close()
	out := <-done
	if exit != 0 {
		t.Fatalf("exit=%d, args=%v, out=%s", exit, args, out)
	}
	return out
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
