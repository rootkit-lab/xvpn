// Command devtool-e2e é um cliente IPC de linha de comando para o helper
// (via devtool-helper ou o helper embutido no binário Wails), usado para
// testar enrollment/connect/disconnect/status ponta a ponta sem precisar de
// uma GUI — ver ROADMAP.md Fase 4.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/rootkit-lab/xvpn/client/internal/config"
	"github.com/rootkit-lab/xvpn/client/internal/helper"
	"github.com/rootkit-lab/xvpn/client/internal/ipc"
)

func main() {
	client, err := ipc.Dial()
	if err != nil {
		fmt.Println("dial error:", err)
		os.Exit(1)
	}
	defer client.Close()

	if len(os.Args) < 2 {
		fmt.Println("uso: testtool <enroll|connect|disconnect|status|get-prefs|set-prefs|logs> [args]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "enroll":
		req := helper.EnrollRequest{ServerBaseURL: os.Args[2], InviteToken: os.Args[3], DeviceName: os.Args[4]}
		if len(os.Args) > 5 {
			mtu, _ := strconv.Atoi(os.Args[5])
			req.MTU = mtu
		}
		var resp helper.EnrollResponse
		err := client.Call(ipc.MethodEnroll, req, &resp)
		if err != nil {
			fmt.Println("enroll error:", err)
			os.Exit(1)
		}
		fmt.Printf("enrolled: %+v\n", resp)
	case "connect":
		if err := client.Call(ipc.MethodConnect, nil, nil); err != nil {
			fmt.Println("connect error:", err)
			os.Exit(1)
		}
		fmt.Println("connected")
	case "disconnect":
		if err := client.Call(ipc.MethodDisconnect, nil, nil); err != nil {
			fmt.Println("disconnect error:", err)
			os.Exit(1)
		}
		fmt.Println("disconnected")
	case "status":
		var resp helper.StatusResponse
		if err := client.Call(ipc.MethodStatus, nil, &resp); err != nil {
			fmt.Println("status error:", err)
			os.Exit(1)
		}
		raw, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(raw))
	case "get-prefs":
		var prefs config.Preferences
		if err := client.Call(ipc.MethodGetPreferences, nil, &prefs); err != nil {
			fmt.Println("get-prefs error:", err)
			os.Exit(1)
		}
		raw, _ := json.MarshalIndent(prefs, "", "  ")
		fmt.Println(string(raw))
	case "set-prefs":
		// uso: testtool set-prefs <kill_switch:0|1> <split_tunnel:0|1> <auto_reconnect:0|1>
		if len(os.Args) < 5 {
			fmt.Println("uso: testtool set-prefs <kill_switch:0|1> <split_tunnel:0|1> <auto_reconnect:0|1>")
			os.Exit(1)
		}
		prefs := config.Preferences{
			KillSwitch:    os.Args[2] == "1",
			SplitTunnel:   os.Args[3] == "1",
			AutoReconnect: os.Args[4] == "1",
		}
		var resp config.Preferences
		if err := client.Call(ipc.MethodSetPreferences, prefs, &resp); err != nil {
			fmt.Println("set-prefs error:", err)
			os.Exit(1)
		}
		raw, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(raw))
	case "logs":
		var resp struct {
			Lines []string `json:"lines"`
		}
		if err := client.Call(ipc.MethodGetLogs, nil, &resp); err != nil {
			fmt.Println("logs error:", err)
			os.Exit(1)
		}
		for _, line := range resp.Lines {
			fmt.Println(line)
		}
	default:
		fmt.Println("comando desconhecido:", os.Args[1])
		os.Exit(1)
	}
}
