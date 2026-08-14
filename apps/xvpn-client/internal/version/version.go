// Package version expõe a versão semântica do cliente, injetada em build
// via -ldflags (ver build/linux/Taskfile.yml e build/windows/Taskfile.yml).
// Em desenvolvimento sem ldflags, cai para "dev".
package version

// Version é substituída em builds de produção, por exemplo:
//
//	-ldflags="-X github.com/rootkit-lab/xvpn/client/internal/version.Version=0.1.0"
var Version = "dev"

// String retorna a versão do binário (semântica ou "dev").
func String() string {
	return Version
}
