//go:build linux

package intranet

// HostsPath é o arquivo de hosts do sistema (helper privilegiado).
func HostsPath() string { return "/etc/hosts" }
