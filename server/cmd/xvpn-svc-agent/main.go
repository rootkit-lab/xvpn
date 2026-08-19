// xvpn-svc-agent aplica o estado desejado de serviços gerenciados num
// peer da malha (Fase 43). Não roda no PID do xvpn-server. Só fala com
// 10.66.66.1:8080 (VPN). Precisa de root para apt/systemctl.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/provision"
)

func main() {
	base := strings.TrimRight(env("XVPN_SVC_URL", "http://10.66.66.1:8080"), "/")
	token := os.Getenv("XVPN_SVC_TOKEN")
	if token == "" {
		log.Fatal("XVPN_SVC_TOKEN é obrigatório")
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	runner := provision.NewSvcRunner()
	log.Printf("xvpn-svc-agent polling %s", base)
	for {
		items, err := desired(client, base, token)
		if err != nil {
			log.Printf("desired: %v", err)
			time.Sleep(8 * time.Second)
			continue
		}
		for _, item := range items {
			raw, err := json.Marshal(provision.SvcSpec{
				Action: item.Action, Slug: item.Slug, Kind: item.Kind,
				Bind: item.Bind, Port: item.Port, Password: item.Password,
				Backends: item.Backends,
			})
			if err != nil {
				continue
			}
			status, msg := "ready", ""
			if err := provision.ApplyService(runner, bytes.NewReader(raw)); err != nil {
				status, msg = "error", err.Error()
				log.Printf("%s: %v", item.Slug, err)
			} else if item.Action == "stop" {
				status = "stopped"
			}
			if err := report(client, base, token, item.ID, status, msg); err != nil {
				log.Printf("status %s: %v", item.Slug, err)
			}
		}
		time.Sleep(8 * time.Second)
	}
}

type desiredItem struct {
	ID       uint     `json:"id"`
	Slug     string   `json:"slug"`
	Kind     string   `json:"kind"`
	Bind     string   `json:"bind"`
	Port     uint16   `json:"port"`
	Password string   `json:"password"`
	Backends []string `json:"backends"`
	Action   string   `json:"action"`
}

func desired(c *http.Client, base, token string) ([]desiredItem, error) {
	req, err := http.NewRequest(http.MethodGet, base+"/api/svc/desired", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("HTTP %d %s", res.StatusCode, bytes.TrimSpace(b))
	}
	var out struct {
		Items []desiredItem `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func report(c *http.Client, base, token string, id uint, status, msg string) error {
	body, err := json.Marshal(map[string]string{"status": status, "error": msg})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/svc/%d/status", base, id), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("HTTP %d %s", res.StatusCode, bytes.TrimSpace(b))
	}
	return nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
