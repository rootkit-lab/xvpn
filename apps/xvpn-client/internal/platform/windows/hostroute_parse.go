package windows

import (
	"fmt"
	"net"
	"strings"
)

func parseIPv4DefaultGateway(routePrint string) (net.IP, error) {
	for _, line := range strings.Split(routePrint, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != "0.0.0.0" || fields[1] != "0.0.0.0" {
			continue
		}
		gw := net.ParseIP(fields[2])
		if gw == nil || gw.IsUnspecified() {
			continue
		}
		if v4 := gw.To4(); v4 != nil {
			return v4, nil
		}
	}
	return nil, fmt.Errorf("nenhuma rota padrão IPv4 encontrada (necessária para preservar acesso ao servidor)")
}
