package zte

import "testing"

const sampleEthPortXML = `<?xml version="1.0"?>
<ajax_response_xml_root>
  <OBJ_ETHPORT_INFO_ID>
    <Instance>
      <ParaName>_InstID</ParaName><ParaValue>DEV.ETH.IF1</ParaValue>
      <ParaName>WanType</ParaName><ParaValue>-1</ParaValue>
      <ParaName>EthPortAliasName</ParaName><ParaValue>ETH_WAN</ParaValue>
      <ParaName>EthPortMaxSpeed</ParaName><ParaValue>3</ParaValue>
      <ParaName>EthPortSpeed</ParaName><ParaValue>3</ParaValue>
      <ParaName>EthPortUpStream</ParaName><ParaValue>1</ParaValue>
    </Instance>
    <Instance>
      <ParaName>_InstID</ParaName><ParaValue>DEV.ETH.IF2</ParaValue>
      <ParaName>EthPortAliasName</ParaName><ParaValue>LAN1</ParaValue>
      <ParaName>EthPortMaxSpeed</ParaName><ParaValue>3</ParaValue>
      <ParaName>EthPortSpeed</ParaName><ParaValue>7</ParaValue>
      <ParaName>EthPortUpStream</ParaName><ParaValue>0</ParaValue>
    </Instance>
  </OBJ_ETHPORT_INFO_ID>
  <OBJ_ETHPORT_STATE_ID>
    <Instance>
      <ParaName>_InstID</ParaName><ParaValue>DEV.ETH.IF1</ParaValue>
      <ParaName>EthPortRecvRate</ParaName><ParaValue>55.3Kbps</ParaValue>
      <ParaName>EthPortSendRate</ParaName><ParaValue>16.0Kbps</ParaValue>
      <ParaName>EthPortStatus</ParaName><ParaValue>0</ParaValue>
    </Instance>
    <Instance>
      <ParaName>_InstID</ParaName><ParaValue>DEV.ETH.IF2</ParaValue>
      <ParaName>EthPortRecvRate</ParaName><ParaValue>0</ParaValue>
      <ParaName>EthPortSendRate</ParaName><ParaValue>0</ParaValue>
      <ParaName>EthPortStatus</ParaName><ParaValue>1</ParaValue>
    </Instance>
  </OBJ_ETHPORT_STATE_ID>
</ajax_response_xml_root>`

func TestParseEthernetPorts(t *testing.T) {
	ports, err := ParseEthernetPorts([]byte(sampleEthPortXML))
	if err != nil {
		t.Fatalf("ParseEthernetPorts returned error: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}

	wan, err := FindWANPort(ports, "ETH_WAN")
	if err != nil {
		t.Fatalf("FindWANPort returned error: %v", err)
	}
	if wan.DisplayName != "WAN" {
		t.Fatalf("expected display name WAN, got %q", wan.DisplayName)
	}
	if !wan.Connected {
		t.Fatal("expected WAN to be connected")
	}
	if wan.SpeedMbps != 1000 || !wan.SpeedKnown {
		t.Fatalf("expected WAN speed 1000 Mbps, got %d known=%v", wan.SpeedMbps, wan.SpeedKnown)
	}
	if !wan.Healthy(1000) {
		t.Fatal("expected WAN to be healthy at 1000 Mbps threshold")
	}
}

func TestFindWANPortFallsBackToUpstream(t *testing.T) {
	ports, err := ParseEthernetPorts([]byte(sampleEthPortXML))
	if err != nil {
		t.Fatalf("ParseEthernetPorts returned error: %v", err)
	}
	wan, err := FindWANPort(ports, "missing")
	if err != nil {
		t.Fatalf("FindWANPort fallback returned error: %v", err)
	}
	if !wan.Upstream {
		t.Fatal("expected fallback port to be upstream")
	}
}

func TestSpeedIndexToMbps(t *testing.T) {
	tests := map[int]int{
		1: 10,
		2: 100,
		3: 1000,
		4: 2500,
		5: 5000,
		6: 10000,
	}
	for index, want := range tests {
		got, ok := SpeedIndexToMbps(index)
		if !ok || got != want {
			t.Fatalf("SpeedIndexToMbps(%d) = %d, %v; want %d, true", index, got, ok, want)
		}
	}
	if got, ok := SpeedIndexToMbps(7); ok || got != 0 {
		t.Fatalf("SpeedIndexToMbps(7) = %d, %v; want 0, false", got, ok)
	}
}
