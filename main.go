// Comando stresstest realiza testes de carga HTTP e exibe um relatório ao final.
//
// Uso:
//
//	stresstest --url=http://google.com --requests=1000 --concurrency=10
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/stresstest/desafio-stress-test/internal/config"
	"github.com/stresstest/desafio-stress-test/internal/loadtest"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg, err := config.Parse(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "erro: %v\n", err)
		return 2
	}

	fmt.Fprintf(stdout, "Iniciando teste de carga: %d requests para %s com concorrência %d...\n\n",
		cfg.Requests, cfg.URL, cfg.Concurrency)

	report := loadtest.Run(ctx, cfg)
	if err := report.Print(stdout); err != nil {
		fmt.Fprintf(stderr, "erro ao escrever relatório: %v\n", err)
		return 1
	}
	if report.Interrupted {
		return 130
	}
	return 0
}
