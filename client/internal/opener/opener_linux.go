//go:build linux

package opener

import (
	"fmt"
	"os/exec"
)

func openURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

func openSMBShare(host, share string) error {
	return exec.Command("xdg-open", fmt.Sprintf("smb://%s/%s", host, share)).Start()
}
