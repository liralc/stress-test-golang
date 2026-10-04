package loadtest

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stresstest/desafio-stress-test/internal/config"
)

func newConfig(url string, requests, concurrency int) config.Config {
	return config.Config{URL: url, Requests: requests, Concurrency: concurrency, Timeout: 5 * time.Second}
}

func TestRunExactRequestCount(t *testing.T) {
	cases := []struct{ requests, concurrency int }{
		{1, 1},
		{10, 1},
		{100, 10},
		{101, 10}, // não divisível pela concorrência
		{7, 50},   // concorrência maior que o total
		{1000, 37},
	}
	for _, c := range cases {
		var hits atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.Write([]byte("ok"))
		}))

		r := Run(context.Background(), newConfig(srv.URL, c.requests, c.concurrency))
		srv.Close()

		if got := hits.Load(); got != int64(c.requests) {
			t.Errorf("requests=%d concurrency=%d: servidor recebeu %d", c.requests, c.concurrency, got)
		}
		if r.Completed != c.requests || r.OK() != c.requests || r.Failed() != 0 || r.Interrupted {
			t.Errorf("requests=%d concurrency=%d: relatório inesperado %+v", c.requests, c.concurrency, r)
		}
		if want := min(c.requests, c.concurrency); r.Concurrency != want {
			t.Errorf("Concurrency = %d, want %d", r.Concurrency, want)
		}
	}
}

func TestRunStatusDistribution(t *testing.T) {
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Ciclo de 10: 6x200, 2x404, 1x500, 1x429.
		switch n.Add(1) % 10 {
		case 0, 1:
			w.WriteHeader(http.StatusNotFound)
		case 2:
			w.WriteHeader(http.StatusInternalServerError)
		case 3:
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	r := Run(context.Background(), newConfig(srv.URL, 100, 8))

	want := map[int]int{200: 60, 404: 20, 500: 10, 429: 10}
	if len(r.StatusCodes) != len(want) {
		t.Fatalf("StatusCodes = %v, want %v", r.StatusCodes, want)
	}
	for code, count := range want {
		if r.StatusCodes[code] != count {
			t.Errorf("status %d = %d, want %d", code, r.StatusCodes[code], count)
		}
	}
	if r.Completed != 100 {
		t.Errorf("Completed = %d, want 100", r.Completed)
	}
}

func TestRunRespectsConcurrency(t *testing.T) {
	const concurrency = 5
	var inFlight, peak atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}))
	defer srv.Close()

	r := Run(context.Background(), newConfig(srv.URL, 50, concurrency))

	if got := peak.Load(); got != concurrency {
		t.Fatalf("pico de requisições simultâneas = %d, want %d", got, concurrency)
	}
	// 50 requests / 5 simultâneos * 20ms ≈ 200ms; sequencial seria ≈ 1s.
	if r.Duration >= 800*time.Millisecond {
		t.Fatalf("execução não parece concorrente: %s", r.Duration)
	}
}

func TestRunTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	cfg := newConfig(srv.URL, 4, 2)
	cfg.Timeout = 50 * time.Millisecond
	r := Run(context.Background(), cfg)

	if r.Completed != 4 || r.Failed() != 4 || r.Errors["timeout"] != 4 {
		t.Fatalf("esperava 4 timeouts, relatório: %+v", r)
	}
	if len(r.StatusCodes) != 0 {
		t.Fatalf("StatusCodes deveria estar vazio: %v", r.StatusCodes)
	}
}

func TestRunConnectionRefused(t *testing.T) {
	// Reserva uma porta e a libera para garantir que nada esteja escutando.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	r := Run(context.Background(), newConfig("http://"+addr, 10, 3))

	if r.Completed != 10 || r.Failed() != 10 || r.OK() != 0 {
		t.Fatalf("relatório inesperado: %+v", r)
	}
	for msg := range r.Errors {
		if !strings.Contains(msg, "refused") {
			t.Errorf("mensagem de erro inesperada: %q", msg)
		}
	}
}

func TestRunInvalidHTTPResponse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	defer l.Close()

	// Servidor TCP que responde lixo em vez de HTTP.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Read(make([]byte, 1024)) // consome a requisição antes de responder
			conn.Write([]byte("isto não é HTTP\r\n\r\n"))
			conn.Close()
		}
	}()

	r := Run(context.Background(), newConfig("http://"+l.Addr().String(), 5, 2))

	if r.Completed != 5 || r.Failed() != 5 {
		t.Fatalf("esperava 5 falhas, relatório: %+v", r)
	}
}

func TestRunFollowsRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, _ *http.Request) {})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	r := Run(context.Background(), newConfig(srv.URL+"/old", 10, 2))

	if r.OK() != 10 {
		t.Fatalf("esperava 10 respostas 200 após redirect, relatório: %+v", r)
	}
}

func TestRunRedirectLoopCountsLastStatus(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	defer srv.Close()

	r := Run(context.Background(), newConfig(srv.URL, 3, 1))

	if r.StatusCodes[http.StatusFound] != 3 || r.Failed() != 0 {
		t.Fatalf("loop de redirect deveria contar como 302, relatório: %+v", r)
	}
	if got := hits.Load(); got != 3*maxRedirects {
		t.Fatalf("servidor recebeu %d requisições, want %d", got, 3*maxRedirects)
	}
}

func TestRunCancellation(t *testing.T) {
	var started atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	done := make(chan Report, 1)
	go func() { done <- Run(ctx, newConfig(srv.URL, 1000, 4)) }()

	select {
	case r := <-done:
		if !r.Interrupted {
			t.Fatalf("relatório deveria indicar interrupção: %+v", r)
		}
		if r.Completed != 0 || r.Failed() != 0 {
			t.Fatalf("requisições canceladas não devem ser contadas: %+v", r)
		}
		if got := started.Load(); got > 4 {
			t.Fatalf("novas requisições iniciadas após cancelamento: %d", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run não retornou após cancelamento")
	}
}
