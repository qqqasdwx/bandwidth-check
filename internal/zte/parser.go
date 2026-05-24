package zte

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type PortStatus struct {
	InstID       string
	Alias        string
	DisplayName  string
	Upstream     bool
	Connected    bool
	SpeedIndex   int
	SpeedMbps    int
	SpeedKnown   bool
	MaxSpeedMbps int
	SendRate     string
	RecvRate     string
	WanType      string
}

func (p PortStatus) Healthy(minSpeedMbps int) bool {
	return p.Connected && p.SpeedKnown && p.SpeedMbps >= minSpeedMbps
}

func (p PortStatus) SpeedText() string {
	if !p.SpeedKnown {
		return "unknown"
	}
	return fmt.Sprintf("%d Mbps", p.SpeedMbps)
}

func ParseEthernetPorts(body []byte) ([]PortStatus, error) {
	info, err := parseObject(body, "OBJ_ETHPORT_INFO_ID")
	if err != nil {
		return nil, err
	}
	state, err := parseObject(body, "OBJ_ETHPORT_STATE_ID")
	if err != nil {
		return nil, err
	}
	stateByID := make(map[string]map[string]string, len(state))
	for _, item := range state {
		stateByID[item["_InstID"]] = item
	}

	var ports []PortStatus
	for _, item := range info {
		instID := item["_InstID"]
		merged := make(map[string]string, len(item)+len(stateByID[instID]))
		for key, value := range item {
			merged[key] = value
		}
		for key, value := range stateByID[instID] {
			merged[key] = value
		}

		speedIndex, _ := strconv.Atoi(merged["EthPortSpeed"])
		speedMbps, speedKnown := SpeedIndexToMbps(speedIndex)
		maxSpeedIndex, _ := strconv.Atoi(merged["EthPortMaxSpeed"])
		maxSpeedMbps, _ := SpeedIndexToMbps(maxSpeedIndex)
		alias := merged["EthPortAliasName"]
		ports = append(ports, PortStatus{
			InstID:       instID,
			Alias:        alias,
			DisplayName:  displayAlias(alias),
			Upstream:     merged["EthPortUpStream"] == "1",
			Connected:    merged["EthPortStatus"] == "0",
			SpeedIndex:   speedIndex,
			SpeedMbps:    speedMbps,
			SpeedKnown:   speedKnown,
			MaxSpeedMbps: maxSpeedMbps,
			SendRate:     merged["EthPortSendRate"],
			RecvRate:     merged["EthPortRecvRate"],
			WanType:      merged["WanType"],
		})
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("no Ethernet ports found in router response")
	}
	return ports, nil
}

func FindWANPort(ports []PortStatus, alias string) (PortStatus, error) {
	normalizedAlias := normalizePortName(alias)
	for _, port := range ports {
		if normalizePortName(port.Alias) == normalizedAlias ||
			normalizePortName(port.DisplayName) == normalizedAlias ||
			normalizePortName(port.InstID) == normalizedAlias {
			return port, nil
		}
	}
	for _, port := range ports {
		if port.Upstream {
			return port, nil
		}
	}
	return PortStatus{}, fmt.Errorf("WAN port %q not found", alias)
}

func SpeedIndexToMbps(index int) (int, bool) {
	switch index {
	case 1:
		return 10, true
	case 2:
		return 100, true
	case 3:
		return 1000, true
	case 4:
		return 2500, true
	case 5:
		return 5000, true
	case 6:
		return 10000, true
	default:
		return 0, false
	}
}

func parseObject(body []byte, objectName string) ([]map[string]string, error) {
	var root xmlRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("parse router XML: %w", err)
	}

	var records []map[string]string
	for _, object := range root.Objects {
		if object.XMLName.Local != objectName {
			continue
		}
		for _, instance := range object.Instances {
			record := make(map[string]string, len(instance.Names))
			for index, name := range instance.Names {
				if index < len(instance.Values) {
					record[strings.TrimSpace(name)] = strings.TrimSpace(instance.Values[index])
				}
			}
			if len(record) > 0 {
				records = append(records, record)
			}
		}
	}
	return records, nil
}

func displayAlias(alias string) string {
	if alias == "ETH_WAN" {
		return "WAN"
	}
	return alias
}

func normalizePortName(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

type xmlRoot struct {
	Objects []xmlObject `xml:",any"`
}

type xmlObject struct {
	XMLName   xml.Name
	Instances []xmlInstance `xml:"Instance"`
}

type xmlInstance struct {
	Names  []string `xml:"ParaName"`
	Values []string `xml:"ParaValue"`
}
