package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/rootkit-lab/xvpn/chat/internal/socialclient"
)

//go:embed all:frontend/dist
var assets embed.FS

var wailsApp *application.App

func emitSocialEvent(ev socialclient.WSEvent) {
	if wailsApp == nil {
		return
	}
	wailsApp.Event.Emit("social:event", ev)
}

func main() {
	svc := NewChatService()
	app := application.New(application.Options{
		Name:        "XVPN Chat",
		Description: "Chat da organização XVPN",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.officeempresa.xvpn-chat",
		},
	})
	wailsApp = app

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "XVPN Chat",
		Width:            920,
		Height:           640,
		BackgroundColour: application.NewRGB(15, 17, 21),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		slog.Error("gui exited", "err", err)
		os.Exit(1)
	}
}
