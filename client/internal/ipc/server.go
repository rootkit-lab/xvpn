package ipc

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
)

// Handler processa os parâmetros de uma chamada RPC e devolve o resultado
// (serializado como JSON) ou um erro (devolvido ao chamador como string —
// texto acionável, nunca um erro genérico, ver .cursor/rules/go-client.mdc).
type Handler func(params json.RawMessage) (any, error)

// Server despacha requisições JSON-RPC recebidas em conexões aceitas de um
// net.Listener (Unix socket ou Named Pipe, conforme a plataforma — ver
// listener_linux.go/listener_windows.go).
type Server struct {
	handlers map[string]Handler
}

func NewServer() *Server {
	return &Server{handlers: make(map[string]Handler)}
}

func (s *Server) Handle(method string, fn Handler) {
	s.handlers[method] = fn
}

// Serve aceita conexões indefinidamente. Cada conexão é tratada numa
// goroutine dedicada; como o helper roda um único Engine/apiclient, o
// acesso concorrente é serializado dentro dos próprios handlers (ver
// internal/helper).
func (s *Server) Serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	for {
		var req Request
		if err := dec.Decode(&req); err != nil {
			return
		}

		resp := Response{JSONRPC: "2.0", ID: req.ID}
		handler, ok := s.handlers[req.Method]
		if !ok {
			resp.Error = fmt.Sprintf("método desconhecido: %q", req.Method)
		} else if result, err := handler(req.Params); err != nil {
			resp.Error = err.Error()
		} else if result != nil {
			raw, err := json.Marshal(result)
			if err != nil {
				resp.Error = fmt.Sprintf("erro interno serializando resposta: %v", err)
			} else {
				resp.Result = raw
			}
		}

		if err := enc.Encode(resp); err != nil {
			log.Printf("ipc: falha ao responder ao cliente: %v", err)
			return
		}
	}
}
