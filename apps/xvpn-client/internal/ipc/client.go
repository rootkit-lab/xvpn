package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
)

// Client é usado pelo processo GUI (sem privilégio) para chamar métodos
// expostos pelo helper. Uma única conexão persistente é reaproveitada para
// todas as chamadas — reconectar automaticamente após queda fica a cargo
// do chamador (ex.: internal/helper/guiservice.go tenta Dial novamente
// antes de reportar "helper indisponível" para a UI).
type Client struct {
	mu     sync.Mutex
	conn   net.Conn
	enc    *json.Encoder
	dec    *json.Decoder
	nextID uint64
}

// newClient envolve uma conexão já estabelecida (por Dial, específico de
// plataforma) num Client pronto para chamadas RPC.
func newClient(conn net.Conn) *Client {
	return &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}
}

// Call envia uma requisição e bloqueia até a resposta correspondente
// chegar. params e result podem ser nil.
func (c *Client) Call(method string, params, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	req := Request{JSONRPC: "2.0", ID: c.nextID, Method: method}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("codificando parâmetros de %q: %w", method, err)
		}
		req.Params = raw
	}

	if err := c.enc.Encode(req); err != nil {
		return fmt.Errorf("enviando requisição ao helper: %w", err)
	}

	var resp Response
	if err := c.dec.Decode(&resp); err != nil {
		return fmt.Errorf("lendo resposta do helper: %w", err)
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	if result != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("decodificando resposta de %q: %w", method, err)
		}
	}
	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
