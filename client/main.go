package main

import (
	"embed"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/rootkit-lab/xvpn/client/internal/applog"
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
	var mainWindow application.Window

	app := application.New(application.Options{
		Name:        "XVPN",
		Description: "Cliente desktop da VPN privada XVPN",
		Services: []application.Service{
			application.NewService(&VPNService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Sem isto, cada clique no ícone do menu/.desktop ou o
		// autostart duplicado abria outro processo (várias janelas/
		// bandejas). Com UniqueID, a segunda invocação só sinaliza a
		// primeira — ver https://v3.wails.io/guides/single-instance
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.officeempresa.xvpn",
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if mainWindow == nil {
					return
				}
				mainWindow.Show()
				mainWindow.Restore()
				mainWindow.Focus()
			},
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "XVPN",
		Width:            420,
		Height:           620,
		BackgroundColour: application.NewRGB(15, 17, 21),
		URL:              "/",
	})
	mainWindow = window

	// Sem isto, o botão "x" da janela dispara o handler padrão do Wails
	// (WindowClosing → destrói a janela de fato, ver
	// pkg/application/webview_window.go). Nesse ponto "Mostrar XVPN" na
	// bandeja não tem mais nada pra mostrar — o app parece ter travado.
	// RegisterHook roda ANTES desse handler padrão (que é um listener via
	// OnWindowEvent, não um hook — hooks sempre são processados primeiro
	// e um Cancel() aqui impede o listener padrão de rodar), então
	// cancelamos o fechamento e só escondemos a janela: fica minimizada
	// na bandeja, pronta para "Mostrar XVPN" trazer de volta.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		window.Hide()
	})

	tray := setupTray(app, window)
	go monitorTray(tray)

	if err := app.Run(); err != nil {
		slog.Error("gui exited", "err", err)
		os.Exit(1)
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
				slog.Warn("tray connect failed", "err", err)
			}
		}()
	})
	disconnectItem := menu.Add("Desconectar").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.Disconnect(); err != nil {
				slog.Warn("tray disconnect failed", "err", err)
			}
		}()
	})
	menu.AddSeparator()
	networkItem := menu.Add("Unidade de rede").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.OpenServerFiles("smb"); err != nil {
				slog.Warn("tray open smb failed", "err", err)
			}
		}()
	})
	filebrowserItem := menu.Add("Arquivos (navegador)").OnClick(func(_ *application.Context) {
		go func() {
			svc := &VPNService{}
			if err := svc.OpenServerFiles("filebrowser"); err != nil {
				slog.Warn("tray open filebrowser failed", "err", err)
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
