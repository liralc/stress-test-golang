// Package config faz o parsing e a validação dos parâmetros de linha de comando.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"time"
)

// DefaultTimeout é o tempo máximo padrão de cada requisição individual.
const DefaultTimeout = 30 * time.Second

// Config reúne os parâmetros de execução do teste de carga.
type Config struct {
	URL         string
	Requests    int
	Concurrency int
	Timeout     time.Duration
}

// Parse interpreta os argumentos (sem o nome do programa) e valida o resultado.
// Mensagens de uso e de erro do pacote flag são escritas em output.
func Parse(args []string, output io.Writer) (Config, error) {
	var cfg Config

	fs := flag.NewFlagSet("stresstest", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&cfg.URL, "url", "", "URL do serviço a ser testado (obrigatório)")
	fs.IntVar(&cfg.Requests, "requests", 0, "número total de requisições (obrigatório, > 0)")
	fs.IntVar(&cfg.Concurrency, "concurrency", 1, "número de chamadas simultâneas (> 0)")
	fs.DurationVar(&cfg.Timeout, "timeout", DefaultTimeout, "tempo máximo de cada requisição (ex: 5s, 500ms)")
	fs.Usage = func() {
		fmt.Fprintln(output, "Uso: stresstest --url=<URL> --requests=<N> --concurrency=<C> [--timeout=<duração>]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("argumentos inesperados: %v", fs.Args())
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate verifica se os parâmetros são coerentes.
func (c Config) Validate() error {
	var errs []error

	if c.URL == "" {
		errs = append(errs, errors.New("--url é obrigatório"))
	} else if u, err := url.Parse(c.URL); err != nil {
		errs = append(errs, fmt.Errorf("--url inválida: %w", err))
	} else if u.Scheme != "http" && u.Scheme != "https" {
		errs = append(errs, fmt.Errorf("--url deve usar http ou https: %q", c.URL))
	} else if u.Host == "" {
		errs = append(errs, fmt.Errorf("--url sem host: %q", c.URL))
	}

	if c.Requests <= 0 {
		errs = append(errs, fmt.Errorf("--requests deve ser maior que zero (recebido %d)", c.Requests))
	}
	if c.Concurrency <= 0 {
		errs = append(errs, fmt.Errorf("--concurrency deve ser maior que zero (recebido %d)", c.Concurrency))
	}
	if c.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("--timeout deve ser maior que zero (recebido %s)", c.Timeout))
	}

	return errors.Join(errs...)
}
