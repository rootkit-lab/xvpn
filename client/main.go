package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/rootkit-lab/xvpn/client/internal/applog"
	"github.com/rootkit-lab/xvpn/client/internal/helper"
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
	applog.Setup("xvpn-client-helper")
	h, err := helper.New()
	if err != nil {
		slog.Error("init failed", "err", err)
		os.Exit(1)
	}
	if err := h.Run(); err != nil {
		slog.Error("helper exited", "err", err)
		os.Exit(1)
	}
}

func runGUI() {
	applog.Setup("xvpn-client-gui")
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

	setupTray(app, window)

	if err := app.Run(); err != nil {
		slog.Error("gui exited", "err", err)
		os.Exit(1)
	}
}

// setupTray registra o ícone de bandeja básico (ROADMAP.md Fase 4):
// clique curto mostra/esconde a janela principal; o menu tem os atalhos de
// conectar/desconectar sem precisar abrir a janela.
func setupTray(app *application.App, window application.Window) {
	tray := app.SystemTray.New()
	tray.SetTooltip("XVPN")
	tray.AttachWindow(window).WindowOffset(4)

	menu := application.NewMenu()
	menu.Add("Mostrar XVPN").OnClick(func(_ *application.Context) {
		window.Show()
		window.Focus()
	})
	menu.AddSeparator()
	menu.Add("Conectar").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.Connect(); err != nil {
				slog.Warn("tray connect failed", "err", err)
			}
		}()
	})
	menu.Add("Desconectar").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.Disconnect(); err != nil {
				slog.Warn("tray disconnect failed", "err", err)
			}
		}()
	})
	menu.AddSeparator()
	menu.Add("Sair").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)
}
