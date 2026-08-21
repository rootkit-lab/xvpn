package store

import "testing"

func TestIsDataNode(t *testing.T) {
	if !IsDataNode("data", "data", DataNodeIPv4) {
		t.Fatal("esperava data node por ipv4/hostname")
	}
	if IsDataNode("edge", "edge", "8.8.8.8") {
		t.Fatal("não deveria ser data node")
	}
	if ManualBitLaunchID("data") != ManualIDPrefix+"data" {
		t.Fatal("ManualBitLaunchID")
	}
}
