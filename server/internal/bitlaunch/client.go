// Package bitlaunch é o cliente HTTP da API BitLaunch (Fase 38).
// O token fica só no VPS (XVPN_BITLAUNCH_TOKEN) — nunca no Git.
package bitlaunch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBase = "https://app.bitlaunch.io/api"

// Server é o objeto devolvido pela API (lista/create/get).
type Server struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	IPv4             string `json:"ipv4"`
	Region           string `json:"region"`
	Size             string `json:"size"`
	Image            string `json:"image"`
	ImageDescription string `json:"imageDescription"`
	Status           string `json:"status"`
	ErrorText        string `json:"errorText"`
}

type CreateOpts struct {
	Name        string
	HostID      int
	HostImageID string
	SizeID      string
	RegionID    string
	SSHKeys     []string
	InitScript  string
}

type RebuildOpts struct {
	HostImageID      string
	ImageDescription string
}

// Client fala com a API. Testes injetam HTTPClient + BaseURL.
type Client struct {
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}

func New(token string) *Client {
	return &Client{
		Token:      token,
		BaseURL:    defaultBase,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) List() ([]Server, error) {
	var out []Server
	if err := c.do(http.MethodGet, "/servers", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Create(opts CreateOpts) (Server, error) {
	body := map[string]any{
		"server": map[string]any{
			"name":        opts.Name,
			"hostID":      opts.HostID,
			"hostImageID": opts.HostImageID,
			"sizeID":      opts.SizeID,
			"regionID":    opts.RegionID,
			"sshKeys":     opts.SSHKeys,
			"initscript":  opts.InitScript,
		},
	}
	var out Server
	if err := c.do(http.MethodPost, "/servers", body, &out); err != nil {
		return Server{}, err
	}
	return out, nil
}

func (c *Client) Destroy(id string) error {
	return c.do(http.MethodDelete, "/servers/"+id, nil, nil)
}

func (c *Client) Rebuild(id string, opts RebuildOpts) error {
	return c.do(http.MethodPost, "/servers/"+id+"/rebuild", map[string]any{
		"hostImageID":      opts.HostImageID,
		"imageDescription": opts.ImageDescription,
	}, nil)
}

func (c *Client) do(method, path string, payload any, dest any) error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("token BitLaunch ausente")
	}
	base := c.BaseURL
	if base == "" {
		base = defaultBase
	}
	var rdr io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(base, "/")+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("bitlaunch %s %s: %s", method, path, res.Status)
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
