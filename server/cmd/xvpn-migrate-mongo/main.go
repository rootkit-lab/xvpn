// One-shot SQLite → Mongo (Fase 28). Não substitui o FileBrowser Quantum.
//
//	XVPN_MONGO_URI=mongodb://xvpn:SENHA@127.0.0.1:27017/xvpn \
//	  ./xvpn-migrate-mongo /opt/xvpn/data/xvpn.db
package main

import (
	"fmt"
	"os"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "uso: xvpn-migrate-mongo <caminho-xvpn.db>\n")
		os.Exit(2)
	}
	uri := os.Getenv("XVPN_MONGO_URI")
	if uri == "" {
		fmt.Fprintf(os.Stderr, "XVPN_MONGO_URI é obrigatório (só 127.0.0.1 em produção)\n")
		os.Exit(2)
	}
	if err := store.ImportSQLiteToMongo(os.Args[1], uri); err != nil {
		fmt.Fprintf(os.Stderr, "migração falhou: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("sqlite → mongo ok")
}
