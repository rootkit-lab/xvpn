package store

import "testing"

func TestIsDataNode(t *testing.T) {
	if !IsDataNode("anything", "data", DataNodeIPv4) {
		t.Fatal("esperava data node por ipv4")
	}
	if !IsDataNode("x", "data", "8.8.8.8") {
		t.Fatal("esperava data node por hostname")
	}
	if IsDataNode("data", "edge", "8.8.8.8") {
		t.Fatal("só o nome display não deve marcar data node")
	}
	if ManualBitLaunchID("data") != ManualIDPrefix+"data" {
		t.Fatal("ManualBitLaunchID")
	}
}
