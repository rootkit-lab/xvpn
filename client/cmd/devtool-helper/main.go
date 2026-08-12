// Command devtool-helper roda o helper privilegiado (internal/helper) sem
// nenhuma dependência de GUI — ao contrário do binário Wails principal
// (client/main.go --helper), que sempre linka libX11/GTK/WebKit2GTK mesmo
// em modo helper. Existe só para testar IPC + engine de túnel em ambientes
// headless (ex.: containers Docker, CI futura) — ver ROADMAP.md Fase 4.
package main

import (
	"log"

	"github.com/rootkit-lab/xvpn/client/internal/helper"
)

func main() {
	h, err := helper.New()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(h.Run())
}
