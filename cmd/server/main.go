package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

var persistConn *sql.Conn
var persistStmt *sql.Stmt

type apiQuote struct {
	Bid string `json:"bid"`
}

func main() {
	db, err := initDB("cotacoes.db")
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer db.Close()
	if persistStmt != nil {
		defer persistStmt.Close()
	}

	http.HandleFunc("/cotacao", cotacaoHandler(db))

	log.Println("server rodando em :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func fetchExternalQuote(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://economia.awesomeapi.com.br/json/last/USD-BRL", nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", context.DeadlineExceeded
		}
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("status não OK da API externa")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed map[string]apiQuote
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}

	for _, v := range parsed {
		if v.Bid != "" {
			return v.Bid, nil
		}
	}
	return "", errors.New("bid não encontrado no JSON")
}

func persistQuote(db *sql.DB, bid string) error {
	ctxDB, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := persistStmt.ExecContext(ctxDB, bid)
	if err != nil {

		if ctxDB.Err() != nil {
			return ctxDB.Err()
		}
		return err
	}

	return nil
}

func cotacaoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctxAPI, cancelAPI := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancelAPI()

		bid, err := fetchExternalQuote(ctxAPI)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Println("timeout ao chamar API externa")
				http.Error(w, "timeout na api externa", http.StatusGatewayTimeout)
				return
			}
			http.Error(w, "erro na chamada externa", http.StatusBadGateway)
			return
		}

		if err := persistQuote(db, bid); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Println("timeout ao persistir no banco")
			} else {
				log.Printf("erro ao inserir no db: %v", err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"bid": bid})
	}
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir db: %w", err)
	}
	return db, nil
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS quotes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  bid TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
 )`)
	if err != nil {
		return fmt.Errorf("erro ao criar tabela: %w", err)
	}
	return nil
}

func initDB(path string) (*sql.DB, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// PRAGMAs para reduzir latência de escrita (trade-offs: durabilidade)
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")
	_, _ = db.Exec("PRAGMA synchronous = OFF;")
	_, _ = db.Exec("PRAGMA temp_store = MEMORY;")
	_, _ = db.Exec("PRAGMA busy_timeout = 1000;")

	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	conn, err := db.Conn(context.Background())
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao obter conn dedicada: %w", err)
	}

	stmt, err := conn.PrepareContext(context.Background(), "INSERT INTO quotes(bid) VALUES(?)")
	if err != nil {
		conn.Close()
		db.Close()
		return nil, fmt.Errorf("erro ao preparar statement na conn: %w", err)
	}

	persistConn = conn
	persistStmt = stmt

	return db, nil
}
