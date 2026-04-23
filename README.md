# Cotação USD-BRL — Instruções de execução

Este repositório contém dois binários em Go:
- `cmd/server/main.go` — servidor HTTP que expõe o endpoint `GET /cotacao` (porta `:8080`).
- `cmd/client/main.go` — cliente que consome `http://localhost:8080/cotacao` e grava `cotacao.txt`.

## Requisitos
- Go (1.20+ recomendado)
- Conexão com a internet para o servidor obter a cotação da API externa.

Arquivos principais: `cmd/server/main.go`, `cmd/client/main.go`, `go.mod`.

## Executar (modo de desenvolvimento)
Abra dois terminais.

1. Rodar o servidor:
  - `go run .\cmd\server\main.go`
    O servidor inicializará em `:8080` e criará o banco SQLite `cotacoes.db` com a tabela `quotes`.

2. Rodar o cliente:
  - `go run .\cmd\client\main.go`
    O cliente faz a requisição com timeout de `300ms`, recebe o campo `bid` e grava `cotacao.txt` com o conteúdo:
  - `Dólar: {valor}`

## Como consultar o banco de dados (`cotacoes.db`)

Requisito: ter o cliente/servidor desligados não é obrigatório (WAL permite leitura concorrente), mas fechar o servidor evita locks inesperados.

- Abrir interativo:
  - `sqlite3 cotacoes.db`
  - Dentro do prompt:
    - `.tables`  — lista tabelas
    - `.headers on`
    - `.mode column`
    - `SELECT * FROM quotes;`
    - `SELECT COUNT(*) FROM quotes;`
    - `SELECT * FROM quotes ORDER BY id DESC LIMIT 20;`
    - `.quit`
