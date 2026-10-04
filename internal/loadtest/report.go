package loadtest

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Report consolida o resultado de uma execução do teste de carga.
type Report struct {
	URL         string
	Planned     int
	Concurrency int
	Duration    time.Duration
	// Completed é o total de requisições realizadas, com ou sem resposta HTTP.
	Completed int
	// StatusCodes conta as requisições que obtiveram resposta, por status HTTP.
	StatusCodes map[int]int
	// Errors conta as requisições sem resposta HTTP, agrupadas por causa.
	Errors map[string]int
	// Interrupted indica que a execução foi cancelada antes de concluir.
	Interrupted bool
}

// OK retorna a quantidade de requisições com status HTTP 200.
func (r Report) OK() int {
	return r.StatusCodes[http.StatusOK]
}

// Failed retorna a quantidade de requisições que não obtiveram resposta HTTP.
func (r Report) Failed() int {
	total := 0
	for _, n := range r.Errors {
		total += n
	}
	return total
}

// RequestsPerSecond retorna a vazão média da execução.
func (r Report) RequestsPerSecond() float64 {
	if r.Duration <= 0 {
		return 0
	}
	return float64(r.Completed) / r.Duration.Seconds()
}

// Print escreve o relatório em formato legível.
func (r Report) Print(w io.Writer) error {
	var b strings.Builder

	line := strings.Repeat("=", 60)
	fmt.Fprintln(&b, line)
	fmt.Fprintln(&b, "                RELATÓRIO DO TESTE DE CARGA")
	fmt.Fprintln(&b, line)
	fmt.Fprintf(&b, "URL:                          %s\n", r.URL)
	fmt.Fprintf(&b, "Concorrência:                 %d\n", r.Concurrency)
	fmt.Fprintf(&b, "Tempo total:                  %s\n", r.Duration.Round(time.Millisecond))
	fmt.Fprintf(&b, "Total de requests realizados: %d de %d\n", r.Completed, r.Planned)
	fmt.Fprintf(&b, "Requests por segundo:         %.2f\n", r.RequestsPerSecond())
	fmt.Fprintf(&b, "Requests com status HTTP 200: %d\n", r.OK())

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Distribuição de outros códigos de status HTTP:")
	codes := make([]int, 0, len(r.StatusCodes))
	for code := range r.StatusCodes {
		if code != http.StatusOK {
			codes = append(codes, code)
		}
	}
	slices.Sort(codes)
	if len(codes) == 0 {
		fmt.Fprintln(&b, "  (nenhum)")
	}
	for _, code := range codes {
		fmt.Fprintf(&b, "  %d %-28s %d\n", code, statusLabel(code), r.StatusCodes[code])
	}

	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Falhas sem resposta HTTP:     %d\n", r.Failed())
	msgs := make([]string, 0, len(r.Errors))
	for msg := range r.Errors {
		msgs = append(msgs, msg)
	}
	slices.Sort(msgs)
	for _, msg := range msgs {
		fmt.Fprintf(&b, "  %d x %s\n", r.Errors[msg], msg)
	}

	if r.Interrupted {
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "ATENÇÃO: execução interrompida; %d requests não foram realizados.\n", r.Planned-r.Completed)
	}
	fmt.Fprintln(&b, line)

	_, err := io.WriteString(w, b.String())
	return err
}

func statusLabel(code int) string {
	if text := http.StatusText(code); text != "" {
		return "(" + text + ")"
	}
	return ""
}
