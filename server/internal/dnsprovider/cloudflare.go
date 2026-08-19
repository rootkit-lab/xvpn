package dnsprovider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBase = "https://api.cloudflare.com/client/v4"

type Account struct {
	ID    string
	Email string
}

type Zone struct {
	ID          string
	Name        string
	Status      string
	NameServers []string
	AccountID   string
}

type Record struct {
	ID      string
	Type    string
	Name    string
	Content string
	TTL     int
	Proxied bool
}

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

func (c *Client) Accounts() ([]Account, error) {
	var rows []cfAccount
	if err := c.do(http.MethodGet, "/accounts", nil, &rows); err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, Account{ID: r.ID, Email: r.Name})
	}
	return out, nil
}

func (c *Client) ListZones() ([]Zone, error) {
	var rows []cfZone
	if err := c.do(http.MethodGet, "/zones?per_page=50", nil, &rows); err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(rows))
	for _, r := range rows {
		out = append(out, zoneFrom(r))
	}
	return out, nil
}

func (c *Client) CreateZone(name, accountID string) (Zone, error) {
	body := map[string]any{"name": name, "type": "full"}
	if accountID != "" {
		body["account"] = map[string]string{"id": accountID}
	}
	var row cfZone
	if err := c.do(http.MethodPost, "/zones", body, &row); err != nil {
		return Zone{}, err
	}
	return zoneFrom(row), nil
}

func (c *Client) ListRecords(zoneID string) ([]Record, error) {
	var rows []cfRecord
	q := "/zones/" + url.PathEscape(zoneID) + "/dns_records?per_page=100"
	if err := c.do(http.MethodGet, q, nil, &rows); err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, r := range rows {
		out = append(out, recordFrom(r))
	}
	return out, nil
}

func (c *Client) CreateRecord(zoneID string, rec Record) (Record, error) {
	var row cfRecord
	if err := c.do(http.MethodPost, "/zones/"+url.PathEscape(zoneID)+"/dns_records", map[string]any{
		"type": rec.Type, "name": rec.Name, "content": rec.Content,
		"ttl": rec.TTL, "proxied": rec.Proxied,
	}, &row); err != nil {
		return Record{}, err
	}
	return recordFrom(row), nil
}

func (c *Client) UpdateRecord(zoneID, id string, rec Record) (Record, error) {
	var row cfRecord
	if err := c.do(http.MethodPatch, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(id), map[string]any{
		"type": rec.Type, "name": rec.Name, "content": rec.Content,
		"ttl": rec.TTL, "proxied": rec.Proxied,
	}, &row); err != nil {
		return Record{}, err
	}
	return recordFrom(row), nil
}

func (c *Client) DeleteRecord(zoneID, id string) error {
	return c.do(http.MethodDelete, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(id), nil, nil)
}

type cfEnvelope struct {
	Success bool            `json:"success"`
	Errors  []cfAPIError    `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type cfAPIError struct {
	Message string `json:"message"`
}

type cfAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cfZone struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	NameServers []string `json:"name_servers"`
	Account     struct {
		ID string `json:"id"`
	} `json:"account"`
}

type cfRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

func zoneFrom(r cfZone) Zone {
	return Zone{ID: r.ID, Name: r.Name, Status: r.Status, NameServers: r.NameServers, AccountID: r.Account.ID}
}

func recordFrom(r cfRecord) Record {
	return Record{ID: r.ID, Type: r.Type, Name: r.Name, Content: r.Content, TTL: r.TTL, Proxied: r.Proxied}
}

func (c *Client) do(method, path string, payload any, dest any) error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("token Cloudflare ausente")
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
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	var env cfEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if res.StatusCode >= 300 {
			return fmt.Errorf("cloudflare %s %s: %s", method, path, res.Status)
		}
		return err
	}
	if !env.Success || res.StatusCode >= 300 {
		msg := res.Status
		if len(env.Errors) > 0 && env.Errors[0].Message != "" {
			msg = env.Errors[0].Message
		}
		return fmt.Errorf("cloudflare %s %s: %s", method, path, msg)
	}
	if dest == nil || len(env.Result) == 0 || string(env.Result) == "null" {
		return nil
	}
	return json.Unmarshal(env.Result, dest)
}
