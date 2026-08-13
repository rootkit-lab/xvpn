package main

import (
	"embed"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/rootkit-lab/xvpn/client/internal/helper"
	"github.com/rootkit-lab/xvpn/client/internal/trayicons"
)

// Wails usa o pacote `embed` do Go para embutir os arquivos do frontend
// (server/web/dist depois de `npm run build`) no binário.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Um único binário serve dois papéis (ver PLAN.md §7.3 e
	// .cursor/rules/go-client.mdc): rodando com --helper, é o processo
	// privilegiado que fala com TUN/rotas (instalado como serviço de
	// sistema — ver deploy/systemd/xvpn-client-helper.service); sem essa
	// flag, é a GUI Wails sem privilégio, que só conversa com o helper via
	// IPC.
	if len(os.Args) > 1 && os.Args[1] == "--helper" {
		runHelper()
		return
	}
	runGUI()
}

func runHelper() {
	h, err := helper.New()
	if err != nil {
		log.Fatalf("xvpn-client-helper: falha ao inicializar: %v", err)
	}
	if err := h.Run(); err != nil {
		log.Fatalf("xvpn-client-helper: %v", err)
	}
}

func runGUI() {
	app := application.New(application.Options{
		Name:        "XVPN",
		Description: "Cliente desktop da VPN privada XVPN",
		Services: []application.Service{
			application.NewService(&VPNService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "XVPN",
		Width:            420,
		Height:           620,
		BackgroundColour: application.NewRGB(15, 17, 21),
		URL:              "/",
	})

	tray := setupTray(app, window)
	go monitorTray(tray)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// trayHandles guarda as referências que monitorTray precisa atualizar
// periodicamente (ícone/tooltip conforme o status, itens habilitados só
// quando fazem sentido) — ver ROADMAP.md Fase 6 ("ícone de bandeja
// completo").
type trayHandles struct {
	tray            *application.SystemTray
	menu            *application.Menu
	connectItem     *application.MenuItem
	disconnectItem  *application.MenuItem
	networkItem     *application.MenuItem
	filebrowserItem *application.MenuItem
}

// setupTray registra o ícone de bandeja: clique curto mostra/esconde a
// janela principal; o menu tem atalhos de conectar/desconectar e de
// arquivos do servidor (Fase 5) sem precisar abrir a janela. O estado
// visual (ícone, tooltip, itens habilitados) é atualizado por
// monitorTray, não aqui.
func setupTray(app *application.App, window application.Window) *trayHandles {
	tray := app.SystemTray.New()
	tray.SetIcon(trayicons.Disconnected)
	tray.SetTooltip("XVPN — desconectado")
	tray.AttachWindow(window).WindowOffset(4)

	menu := application.NewMenu()
	menu.Add("Mostrar XVPN").OnClick(func(_ *application.Context) {
		window.Show()
		window.Focus()
	})
	menu.AddSeparator()
	connectItem := menu.Add("Conectar").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.Connect(); err != nil {
				log.Printf("tray: falha ao conectar: %v", err)
			}
		}()
	})
	disconnectItem := menu.Add("Desconectar").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.Disconnect(); err != nil {
				log.Printf("tray: falha ao desconectar: %v", err)
			}
		}()
	})
	menu.AddSeparator()
	networkItem := menu.Add("Unidade de rede").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.OpenServerFiles("smb"); err != nil {
				log.Printf("tray: falha ao abrir unidade de rede: %v", err)
			}
		}()
	})
	filebrowserItem := menu.Add("Arquivos (navegador)").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.OpenServerFiles("filebrowser"); err != nil {
				log.Printf("tray: falha ao abrir FileBrowser: %v", err)
			}
		}()
	})
	menu.AddSeparator()
	menu.Add("Sair").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	// Estado inicial: sem status ainda consultado, trata como
	// desconectado/desabilitado até a primeira rodada de monitorTray.
	connectItem.SetEnabled(true)
	disconnectItem.SetEnabled(false)
	networkItem.SetEnabled(false)
	filebrowserItem.SetEnabled(false)
	menu.Update()

	return &trayHandles{
		tray:            tray,
		menu:            menu,
		connectItem:     connectItem,
		disconnectItem:  disconnectItem,
		networkItem:     networkItem,
		filebrowserItem: filebrowserItem,
	}
}

// trayMonitorInterval balanceia responsividade visual (o usuário vê o
// ícone mudar rápido depois de clicar Conectar) contra custo de ficar
// batendo no socket IPC o tempo todo — mesma ordem de grandeza do
// polling que a janela principal já faz (ver frontend/src/App.tsx).
const trayMonitorInterval = 3 * time.Second

// monitorTray roda pelo resto da vida do processo GUI, mantendo o ícone
// da bandeja, o tooltip e quais itens do menu estão habilitados
// sincronizados com o status real do túnel — inclusive durante uma
// reconexão automática (ver internal/helper/reconnect.go), que fica
// visualmente diferente de "conectado" e de "desconectado".
func monitorTray(h *trayHandles) {
	svc := &VPNService{}
	ticker := time.NewTicker(trayMonitorInterval)
	defer ticker.Stop()
	for {
		status, err := svc.Status()
		if err != nil {
			applyTrayStatus(h, StatusView{})
		} else {
			applyTrayStatus(h, status)
		}
		<-ticker.C
	}
}

func applyTrayStatus(h *trayHandles, status StatusView) {
	var icon []byte
	tooltip := "XVPN"

	switch {
	case !status.HelperReachable:
		icon = trayicons.Error
		tooltip = "XVPN — serviço indisponível"
	case status.Reconnecting:
		icon = trayicons.Reconnecting
		tooltip = "XVPN — reconectando (tentativa " + strconv.Itoa(status.ReconnectAttempt+1) + ")"
	case status.Connected:
		icon = trayicons.Connected
		tooltip = "XVPN — conectado"
		if status.AssignedIP != "" {
			tooltip += " (" + status.AssignedIP + ")"
		}
		if status.KillSwitchActive {
			tooltip += " · kill switch ativo"
		}
	default:
		icon = trayicons.Disconnected
		tooltip = "XVPN — desconectado"
	}

	h.tray.SetIcon(icon)
	h.tray.SetTooltip(tooltip)
	h.connectItem.SetEnabled(!status.Connected)
	h.disconnectItem.SetEnabled(status.Connected)
	h.networkItem.SetEnabled(status.Connected)
	h.filebrowserItem.SetEnabled(status.Connected)
	h.menu.Update()
}
