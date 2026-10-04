package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	var stdout, stderr strings.Builder
	code := run(context.Background(), []string{"--url=" + srv.URL, "--requests=20", "--concurrency=4"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"Total de requests realizados: 20 de 20", "Requests com status HTTP 200: 20"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("saída não contém %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunInvalidArgs(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run(context.Background(), []string{"--requests=10"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--url é obrigatório") {
		t.Fatalf("stderr inesperado: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout deveria estar vazio: %s", stdout.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}
