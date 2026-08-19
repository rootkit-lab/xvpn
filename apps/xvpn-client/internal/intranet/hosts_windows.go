//go:build windows

package intranet

import "os"

// HostsPath é o arquivo de hosts do Windows (helper privilegiado).
func HostsPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return root + `\System32\drivers\etc\hosts`
}
