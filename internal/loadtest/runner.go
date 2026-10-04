// Package loadtest executa o teste de carga HTTP e consolida os resultados.
package loadtest

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/stresstest/desafio-stress-test/internal/config"
)

// tally acumula os resultados de um único worker. Cada worker possui o seu,
// eliminando estado compartilhado e locks durante a execução.
type tally struct {
	completed   int
	statusCodes map[int]int
	errors      map[string]int
}

func newTally() *tally {
	return &tally{statusCodes: make(map[int]int), errors: make(map[string]int)}
}

// Run dispara exatamente cfg.Requests requisições GET contra cfg.URL usando
// cfg.Concurrency workers simultâneos. Se ctx for cancelado, nenhuma nova
// requisição é iniciada, as que estão em andamento são abortadas e o relatório
// parcial é retornado com Interrupted = true.
func Run(ctx context.Context, cfg config.Config) Report {
	client := newHTTPClient(cfg)
	defer client.CloseIdleConnections()

	workers := min(cfg.Concurrency, cfg.Requests)

	// Canal pré-preenchido com exatamente cfg.Requests "fichas" e fechado:
	// cada ficha consumida corresponde a uma requisição, o que garante o total
	// exato sem goroutine produtora. struct{} tem tamanho zero, então o buffer
	// não ocupa memória proporcional ao número de requisições.
	jobs := make(chan struct{}, cfg.Requests)
	for range cfg.Requests {
		jobs <- struct{}{}
	}
	close(jobs)

	tallies := make([]*tally, workers)
	var wg sync.WaitGroup

	start := time.Now()
	for i := range workers {
		t := newTally()
		tallies[i] = t
		wg.Add(1)
		go func() {
			defer wg.Done()
			work(ctx, client, cfg.URL, jobs, t)
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	report := Report{
		URL:         cfg.URL,
		Planned:     cfg.Requests,
		Concurrency: workers,
		Duration:    elapsed,
		StatusCodes: make(map[int]int),
		Errors:      make(map[string]int),
	}
	for _, t := range tallies {
		report.Completed += t.completed
		for code, n := range t.statusCodes {
			report.StatusCodes[code] += n
		}
		for msg, n := range t.errors {
			report.Errors[msg] += n
		}
	}
	report.Interrupted = report.Completed < report.Planned && ctx.Err() != nil

	return report
}

func work(ctx context.Context, client *http.Client, target string, jobs <-chan struct{}, t *tally) {
	for range jobs {
		if ctx.Err() != nil {
			return
		}
		status, err := doRequest(ctx, client, target)
		if err != nil {
			// Requisição abortada pelo cancelamento do próprio teste não é uma
			// falha do serviço testado: não entra no relatório.
			if ctx.Err() != nil {
				return
			}
			t.errors[describeError(err)]++
		} else {
			t.statusCodes[status]++
		}
		t.completed++
	}
}

func doRequest(ctx context.Context, client *http.Client, target string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Consumir o corpo permite reutilizar a conexão (keep-alive) e reflete o
	// custo real de uma requisição completa. O status já foi recebido, então
	// uma falha na leitura do corpo não invalida o código HTTP obtido.
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

func newHTTPClient(cfg config.Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = cfg.Concurrency
	transport.MaxIdleConnsPerHost = cfg.Concurrency

	return &http.Client{
		Transport:     transport,
		Timeout:       cfg.Timeout,
		CheckRedirect: limitRedirects,
	}
}

// maxRedirects é o número máximo de requisições por cadeia de redirecionamento,
// o mesmo limite padrão do net/http.
const maxRedirects = 10

// limitRedirects segue redirecionamentos (ex: http://google.com responde 301)
// até maxRedirects. Ao atingir o limite, em vez de descartar a resposta com
// erro, devolve a última resposta recebida para que seu status 3xx seja
// contabilizado: o servidor respondeu via HTTP, não houve falha de comunicação.
func limitRedirects(_ *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return http.ErrUseLastResponse
	}
	return nil
}

// describeError reduz o erro a uma mensagem estável, usada para agrupar falhas
// semelhantes no relatório.
func describeError(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}
