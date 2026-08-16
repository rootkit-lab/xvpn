package ipc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
)

// Handler processa os parâmetros de uma chamada RPC e devolve o resultado
// (serializado como JSON) ou um erro (devolvido ao chamador como string —
// texto acionável, nunca um erro genérico, ver .cursor/rules/go-client.mdc).
type Handler func(params json.RawMessage) (any, error)

// PeerHandler é um Handler que também recebe a identidade do processo
// no outro extremo (SO_PEERCRED no Linux). Use para operações
// privilegiadas que não podem confiar em uid/gid no JSON.
type PeerHandler func(params json.RawMessage, peer Peer) (any, error)

// Server despacha requisições JSON-RPC recebidas em conexões aceitas de um
// net.Listener (Unix socket ou Named Pipe, conforme a plataforma — ver
// listener_linux.go/listener_windows.go).
type Server struct {
	handlers     map[string]Handler
	peerHandlers map[string]PeerHandler
}

func NewServer() *Server {
	return &Server{
		handlers:     make(map[string]Handler),
		peerHandlers: make(map[string]PeerHandler),
	}
}

func (s *Server) Handle(method string, fn Handler) {
	s.handlers[method] = fn
}

func (s *Server) HandlePeer(method string, fn PeerHandler) {
	s.peerHandlers[method] = fn
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
		if ph, ok := s.peerHandlers[req.Method]; ok {
			peer, err := peerFromConn(conn)
			if err != nil {
				resp.Error = err.Error()
			} else {
				s.applyHandler(&resp, func() (any, error) { return ph(req.Params, peer) })
			}
		} else if handler, ok := s.handlers[req.Method]; ok {
			s.applyHandler(&resp, func() (any, error) { return handler(req.Params) })
		} else {
			resp.Error = fmt.Sprintf("método desconhecido: %q", req.Method)
		}

		if err := enc.Encode(resp); err != nil {
			slog.Warn("ipc encode failed", "err", err)
			return
		}
	}
}

func (s *Server) applyHandler(resp *Response, fn func() (any, error)) {
	result, err := fn()
	if err != nil {
		resp.Error = err.Error()
		return
	}
	if result == nil {
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		resp.Error = fmt.Sprintf("erro interno serializando resposta: %v", err)
		return
	}
	resp.Result = raw
}
