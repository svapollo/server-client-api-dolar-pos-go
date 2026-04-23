package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if err := runClient(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println("timeout ao receber resposta do servidor")
			return
		}
		log.Fatalf("%v", err)
	}
}

type bidResp struct {
	Bid string `json:"bid"`
}

func buildRequest(ctx context.Context, url string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
}

func doRequest(req *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGatewayTimeout {
		return nil, context.DeadlineExceeded
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}

	return io.ReadAll(resp.Body)
}

func parseBid(body []byte) (string, error) {
	var r bidResp
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.Bid == "" {
		return "", errors.New("bid vazio recebido")
	}
	return r.Bid, nil
}

func saveBidToFile(bid, path string) error {
	txt := fmt.Sprintf("Dólar: %s", bid)
	return os.WriteFile(path, []byte(txt), 0644)
}

func runClient() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req, err := buildRequest(ctx, "http://localhost:8080/cotacao")
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %w", err)
	}

	body, err := doRequest(req)
	if err != nil {
		return err
	}

	bid, err := parseBid(body)
	if err != nil {
		return err
	}

	if err := saveBidToFile(bid, "cotacao.txt"); err != nil {
		return fmt.Errorf("erro ao escrever arquivo: %w", err)
	}

	log.Println("cotação salva em cotacao.txt")
	return nil
}
