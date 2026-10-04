package loadtest

import (
	"strings"
	"testing"
	"time"
)

func TestReportPrint(t *testing.T) {
	r := Report{
		URL:         "http://example.com",
		Planned:     100,
		Concurrency: 10,
		Duration:    2 * time.Second,
		Completed:   100,
		StatusCodes: map[int]int{200: 80, 500: 5, 404: 10},
		Errors:      map[string]int{"timeout": 5},
	}

	var b strings.Builder
	if err := r.Print(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()

	for _, want := range []string{
		"URL:                          http://example.com",
		"Tempo total:                  2s",
		"Total de requests realizados: 100 de 100",
		"Requests por segundo:         50.00",
		"Requests com status HTTP 200: 80",
		"404 (Not Found)",
		"500 (Internal Server Error)",
		"Falhas sem resposta HTTP:     5",
		"5 x timeout",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("saída não contém %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "404") > strings.Index(out, "500") {
		t.Errorf("códigos de status deveriam estar ordenados:\n%s", out)
	}
	if strings.Contains(out, "200 (OK)") {
		t.Errorf("200 não deveria aparecer em 'outros códigos':\n%s", out)
	}
	if strings.Contains(out, "interrompida") {
		t.Errorf("não deveria indicar interrupção:\n%s", out)
	}
}

func TestReportPrintEmptyAndInterrupted(t *testing.T) {
	r := Report{
		URL:         "http://example.com",
		Planned:     10,
		Completed:   3,
		StatusCodes: map[int]int{200: 3},
		Errors:      map[string]int{},
		Interrupted: true,
	}

	var b strings.Builder
	if err := r.Print(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()

	for _, want := range []string{"(nenhum)", "Requests por segundo:         0.00", "7 requests não foram realizados"} {
		if !strings.Contains(out, want) {
			t.Errorf("saída não contém %q:\n%s", want, out)
		}
	}
}
