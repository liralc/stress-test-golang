package config

import (
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	cfg, err := Parse([]string{"--url=http://google.com", "--requests=1000", "--concurrency=10"}, io.Discard)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	want := Config{URL: "http://google.com", Requests: 1000, Concurrency: 10, Timeout: DefaultTimeout}
	if cfg != want {
		t.Fatalf("config = %+v, want %+v", cfg, want)
	}
}

func TestParseCustomTimeoutAndSingleDash(t *testing.T) {
	cfg, err := Parse([]string{"-url", "https://example.com/x", "-requests", "5", "-concurrency", "2", "-timeout", "250ms"}, io.Discard)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.Timeout != 250*time.Millisecond {
		t.Fatalf("timeout = %s, want 250ms", cfg.Timeout)
	}
}

func TestParseHelp(t *testing.T) {
	var out strings.Builder
	_, err := Parse([]string{"--help"}, &out)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("err = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(out.String(), "--url") {
		t.Fatalf("uso não exibido: %q", out.String())
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"sem url", []string{"--requests=1", "--concurrency=1"}, "--url é obrigatório"},
		{"esquema inválido", []string{"--url=ftp://host", "--requests=1"}, "http ou https"},
		{"sem host", []string{"--url=http://", "--requests=1"}, "sem host"},
		{"url malformada", []string{"--url=http://[::1", "--requests=1"}, "--url inválida"},
		{"requests zero", []string{"--url=http://x", "--requests=0"}, "--requests deve ser maior que zero"},
		{"requests negativo", []string{"--url=http://x", "--requests=-3"}, "--requests deve ser maior que zero"},
		{"concurrency zero", []string{"--url=http://x", "--requests=1", "--concurrency=0"}, "--concurrency deve ser maior que zero"},
		{"timeout zero", []string{"--url=http://x", "--requests=1", "--timeout=0s"}, "--timeout deve ser maior que zero"},
		{"requests não numérico", []string{"--url=http://x", "--requests=abc"}, "invalid value"},
		{"flag desconhecida", []string{"--foo=1"}, "not defined"},
		{"argumento posicional", []string{"--url=http://x", "--requests=1", "extra"}, "argumentos inesperados"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.args, io.Discard)
			if err == nil {
				t.Fatal("esperava erro, obteve nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("erro = %q, deveria conter %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReportsAllErrors(t *testing.T) {
	err := Config{}.Validate()
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, want := range []string{"--url", "--requests", "--concurrency", "--timeout"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro %q não menciona %s", err, want)
		}
	}
}
