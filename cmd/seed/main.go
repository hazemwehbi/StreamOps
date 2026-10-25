// Seed crée un catalogue synthétique via l’API publique. Réservé au développement.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if os.Getenv("ALLOW_DEV_SEED") != "true" {
		return fmt.Errorf("définissez ALLOW_DEV_SEED=true pour un environnement de développement isolé")
	}
	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		return fmt.Errorf("ADMIN_TOKEN est obligatoire")
	}
	base := os.Getenv("BASE_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	count := 1000
	if v := os.Getenv("SEED_COUNT"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100000 {
			return fmt.Errorf("SEED_COUNT doit être compris entre 1 et 100000")
		}
		count = n
	}
	client := http.Client{Timeout: 10 * time.Second}
	for i := 0; i < count; i++ {
		body, _ := json.Marshal(map[string]string{"service_key": fmt.Sprintf("load-service-%06d", i), "name": fmt.Sprintf("Service synthétique %d", i), "environment": "staging"})
		req, err := http.NewRequest("POST", strings.TrimRight(base, "/")+"/v1/services", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("échec de la requête de génération des données")
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != 201 && response.StatusCode != 409 {
			return fmt.Errorf("échec de la génération à l’élément %d avec le code HTTP %d", i, response.StatusCode)
		}
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Printf("%d services synthétiques créés ou conservés.\n", count)
	return nil
}
