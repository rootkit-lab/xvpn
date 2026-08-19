package ipc

// Peer é a identidade do processo no outro extremo da conexão IPC.
// No Linux vem de SO_PEERCRED (kernel), nunca do JSON. Handlers
// privilegiados (mount/umount) usam isto, não campos que a GUI possa forjar.
type Peer struct {
	UID int
	GID int
}
