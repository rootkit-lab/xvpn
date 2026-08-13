// Package trayicons embute os ícones de status do ícone de bandeja: bolinha
// verde (conectado), cinza (desconectado), âmbar (reconectando) e vermelha
// (erro/helper indisponível) — ver ROADMAP.md Fase 6 e main.go
// (monitorTray). Gerados via Pillow (círculo sólido 64x64 com contorno
// branco, transparente ao redor) — não são obra de arte, só um indicador
// de status discreto e legível em fundo claro ou escuro.
package trayicons

import _ "embed"

//go:embed connected.png
var Connected []byte

//go:embed disconnected.png
var Disconnected []byte

//go:embed reconnecting.png
var Reconnecting []byte

//go:embed error.png
var Error []byte
