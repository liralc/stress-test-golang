# Stress Test CLI

Ferramenta de linha de comando em Go para fazer testes de carga em serviços web.
Ela envia um número **exato** de requisições HTTP `GET` para uma URL, mantendo um nível fixo de concorrência, e mostra um relatório no console ao final.

Usa apenas a biblioteca padrão do Go, sem dependências externas.

## Parâmetros

| Flag            | Obrigatório | Padrão | Descrição                                          |
|-----------------|-------------|--------|----------------------------------------------------|
| `--url`         | sim         | –      | URL do serviço a ser testado (`http` ou `https`).  |
| `--requests`    | sim         | –      | Número total de requisições (> 0).                 |
| `--concurrency` | não         | `1`    | Número de chamadas simultâneas (> 0).              |
| `--timeout`     | não         | `30s`  | Tempo máximo de cada requisição (ex.: `5s`, `500ms`). |

## Executando com Docker (recomendado)

### 1. Construir a imagem

Na raiz do projeto:

```bash
docker build -t stresstest .
```

O build tem dois estágios: compila em `golang:1.24-alpine` e gera uma imagem final `scratch` com cerca de 8 MB. Essa imagem contém só o binário estático e os certificados raiz necessários para URLs `https://`.

### 2. Rodar o teste

```bash
docker run --rm stresstest --url=http://google.com --requests=1000 --concurrency=10
```

Outros exemplos:

```bash
# Ajuda
docker run --rm stresstest --help

# Timeout por requisição personalizado
docker run --rm stresstest --url=https://example.com --requests=500 --concurrency=50 --timeout=5s

# Testar um serviço rodando na máquina host (Linux)
docker run --rm --network=host stresstest --url=http://localhost:8080 --requests=200 --concurrency=20
```

Para interromper um teste em andamento, use `Ctrl+C`. Isso funciona quando o container foi iniciado com `-it`, por exemplo `docker run --rm -it stresstest ...`. Nesse caso, o relatório parcial é exibido.

## Executando localmente (sem Docker)

Requer Go 1.24 ou mais recente.

```bash
go run . --url=http://google.com --requests=1000 --concurrency=10

# ou gerando o binário
go build -o stresstest .
./stresstest --url=http://google.com --requests=1000 --concurrency=10
```

## Exemplo de relatório

```
Iniciando teste de carga: 1000 requests para http://google.com com concorrência 10...

============================================================
                RELATÓRIO DO TESTE DE CARGA
============================================================
URL:                          http://google.com
Concorrência:                 10
Tempo total:                  12.873s
Total de requests realizados: 1000 de 1000
Requests por segundo:         77.68
Requests com status HTTP 200: 990

Distribuição de outros códigos de status HTTP:
  404 (Not Found)                  6
  500 (Internal Server Error)      2

Falhas sem resposta HTTP:     2
  2 x timeout
============================================================
```

O relatório tem as seguintes linhas:

- **Tempo total**: duração total da execução.
- **Total de requests realizados**: todas as requisições feitas, com ou sem resposta HTTP.
- **Requests com status HTTP 200**: quantidade de respostas `200 OK`.
- **Distribuição de outros códigos de status HTTP**: contagem de cada status diferente de 200, em ordem crescente.
- **Falhas sem resposta HTTP**: erros de rede, como timeout, conexão recusada ou erro de DNS, agrupados por causa.

Códigos de saída do processo:

- `0`: execução concluída;
- `2`: parâmetros inválidos;
- `130`: execução interrompida (`Ctrl+C` ou `SIGTERM`).

## Como funciona

- **Total exato de requisições:** um canal com buffer recebe exatamente `--requests` "fichas" e é fechado em seguida. Os workers consomem as fichas, e cada ficha vira uma requisição. Por isso o total enviado é sempre exatamente o pedido, mesmo quando ele não é múltiplo da concorrência.
- **Concorrência:** são iniciadas `min(--concurrency, --requests)` goroutines. Assim, nunca há mais requisições simultâneas do que o valor informado.
- **Sem estado compartilhado:** cada worker conta seus próprios resultados, e as contagens são somadas depois de `sync.WaitGroup.Wait()`. Não há mutex no caminho principal da execução.
- **Conexões:** o corpo de cada resposta é lido por completo e fechado, o que permite reaproveitar as conexões (keep-alive). O pool de conexões ociosas é dimensionado de acordo com a concorrência.
- **Redirecionamentos:** são seguidos com o mesmo limite padrão do `net/http` (10 requisições por cadeia). Por exemplo, `http://google.com` responde `301` para `http://www.google.com/`. Se o limite for atingido, conta o status 3xx da última resposta.
- **Cancelamento:** `SIGINT` e `SIGTERM` cancelam um `context.Context`. Nenhuma nova requisição começa depois disso, as que estão em andamento são abortadas e o relatório parcial é exibido.

## Estrutura

```
.
├── main.go                     # ponto de entrada: sinais, parsing e impressão do relatório
├── internal/config/            # parsing e validação das flags
├── internal/loadtest/
│   ├── runner.go               # pool de workers e execução das requisições
│   └── report.go               # agregação e formatação do relatório
├── Dockerfile
└── README.md
```

## Testes

Os testes usam servidores locais (`httptest`) e não acessam a internet.

```bash
go test ./...
go test -race ./...
go vet ./...
```

Eles cobrem:

- contagem exata de requisições, inclusive com totais não múltiplos da concorrência e com concorrência maior que o total;
- distribuição de status;
- limite de concorrência;
- timeout;
- conexão recusada;
- resposta que não é HTTP válido;
- redirecionamentos;
- cancelamento;
- validação de parâmetros;
- formatação do relatório.
