package model

import (
	"encoding/json"
	"testing"
)

func TestOltConfig_Initialization(t *testing.T) {
	config := OltConfig{
		BaseOID:                   "1.3.6.1.4.1",
		OnuIDNameOID:              "1.3.6.1.4.1.1",
		OnuTypeOID:                "1.3.6.1.4.1.2",
		OnuSerialNumberOID:        "1.3.6.1.4.1.3",
		OnuRxPowerOID:             "1.3.6.1.4.1.4",
		OnuTxPowerOID:             "1.3.6.1.4.1.5",
		OnuStatusOID:              "1.3.6.1.4.1.6",
		OnuIPAddressOID:           "1.3.6.1.4.1.7",
		OnuDescriptionOID:         "1.3.6.1.4.1.8",
		OnuLastOnlineOID:          "1.3.6.1.4.1.9",
		OnuLastOfflineOID:         "1.3.6.1.4.1.10",
		OnuLastOfflineReasonOID:   "1.3.6.1.4.1.11",
		OnuGponOpticalDistanceOID: "1.3.6.1.4.1.12",
	}

	if config.BaseOID != "1.3.6.1.4.1" {
		t.Errorf("Expected BaseOID '1.3.6.1.4.1', got '%s'", config.BaseOID)
	}

	if config.OnuTypeOID != "1.3.6.1.4.1.2" {
		t.Errorf("Expected OnuTypeOID '1.3.6.1.4.1.2', got '%s'", config.OnuTypeOID)
	}
}

func TestONUInfo_JSONMarshaling(t *testing.T) {
	info := ONUInfo{
		ID:   "123",
		Name: "Test ONU",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal ONUInfo: %v", err)
	}

	// Unmarshal back
	var unmarshaled ONUInfo
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal ONUInfo: %v", err)
	}

	if unmarshaled.ID != info.ID {
		t.Errorf("Expected ID '%s', got '%s'", info.ID, unmarshaled.ID)
	}

	if unmarshaled.Name != info.Name {
		t.Errorf("Expected Name '%s', got '%s'", info.Name, unmarshaled.Name)
	}
}

func TestONUInfoPerBoard_JSONMarshaling(t *testing.T) {
	info := ONUInfoPerBoard{
		Board:        1,
		PON:          8,
		ID:           5,
		Name:         "Customer A",
		OnuType:      "F670LV7.1",
		SerialNumber: "ZTEGC123456",
		RXPower:      "-20.5",
		Status:       "Online",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal ONUInfoPerBoard: %v", err)
	}

	// Unmarshal back
	var unmarshaled ONUInfoPerBoard
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal ONUInfoPerBoard: %v", err)
	}

	if unmarshaled.Board != info.Board {
		t.Errorf("Expected Board %d, got %d", info.Board, unmarshaled.Board)
	}

	if unmarshaled.PON != info.PON {
		t.Errorf("Expected PON %d, got %d", info.PON, unmarshaled.PON)
	}

	if unmarshaled.ID != info.ID {
		t.Errorf("Expected ID %d, got %d", info.ID, unmarshaled.ID)
	}

	if unmarshaled.SerialNumber != info.SerialNumber {
		t.Errorf("Expected SerialNumber '%s', got '%s'", info.SerialNumber, unmarshaled.SerialNumber)
	}
}

func TestONUCustomerInfo_JSONMarshaling(t *testing.T) {
	info := ONUCustomerInfo{
		Board:                1,
		PON:                  8,
		ID:                   5,
		Name:                 "Customer A",
		Description:          "Test Customer",
		OnuType:              "F670LV7.1",
		SerialNumber:         "ZTEGC123456",
		RXPower:              "-20.5",
		TXPower:              "2.5",
		Status:               "Online",
		IPAddress:            "10.0.0.1",
		LastOnline:           "2024-01-01 10:00:00",
		LastOffline:          "2024-01-01 09:00:00",
		Uptime:               "1 days 2 hours",
		LastDownTimeDuration: "5 minutes",
		LastOfflineReason:    "PowerOff",
		GponOpticalDistance:  "5000",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal ONUCustomerInfo: %v", err)
	}

	// Unmarshal back
	var unmarshaled ONUCustomerInfo
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal ONUCustomerInfo: %v", err)
	}

	if unmarshaled.IPAddress != info.IPAddress {
		t.Errorf("Expected IPAddress '%s', got '%s'", info.IPAddress, unmarshaled.IPAddress)
	}

	if unmarshaled.LastOfflineReason != info.LastOfflineReason {
		t.Errorf("Expected LastOfflineReason '%s', got '%s'", info.LastOfflineReason, unmarshaled.LastOfflineReason)
	}

	if unmarshaled.GponOpticalDistance != info.GponOpticalDistance {
		t.Errorf("Expected GponOpticalDistance '%s', got '%s'", info.GponOpticalDistance, unmarshaled.GponOpticalDistance)
	}
}

func TestOnuID_JSONMarshaling(t *testing.T) {
	onuID := OnuID{
		Board: 1,
		PON:   8,
		ID:    5,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(onuID)
	if err != nil {
		t.Fatalf("Failed to marshal OnuID: %v", err)
	}

	// Unmarshal back
	var unmarshaled OnuID
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal OnuID: %v", err)
	}

	if unmarshaled.Board != onuID.Board {
		t.Errorf("Expected Board %d, got %d", onuID.Board, unmarshaled.Board)
	}

	if unmarshaled.PON != onuID.PON {
		t.Errorf("Expected PON %d, got %d", onuID.PON, unmarshaled.PON)
	}

	if unmarshaled.ID != onuID.ID {
		t.Errorf("Expected ID %d, got %d", onuID.ID, unmarshaled.ID)
	}
}

func TestOnuOnlyID_JSONMarshaling(t *testing.T) {
	onuID := OnuOnlyID{
		ID: 42,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(onuID)
	if err != nil {
		t.Fatalf("Failed to marshal OnuOnlyID: %v", err)
	}

	// Verify JSON structure
	expected := `{"onu_id":42}`
	if string(jsonData) != expected {
		t.Errorf("Expected JSON '%s', got '%s'", expected, string(jsonData))
	}

	// Unmarshal back
	var unmarshaled OnuOnlyID
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal OnuOnlyID: %v", err)
	}

	if unmarshaled.ID != onuID.ID {
		t.Errorf("Expected ID %d, got %d", onuID.ID, unmarshaled.ID)
	}
}

func TestOnuSerialNumber_JSONMarshaling(t *testing.T) {
	sn := OnuSerialNumber{
		Board:        1,
		PON:          8,
		ID:           5,
		SerialNumber: "ZTEGC123456",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(sn)
	if err != nil {
		t.Fatalf("Failed to marshal OnuSerialNumber: %v", err)
	}

	// Unmarshal back
	var unmarshaled OnuSerialNumber
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal OnuSerialNumber: %v", err)
	}

	if unmarshaled.SerialNumber != sn.SerialNumber {
		t.Errorf("Expected SerialNumber '%s', got '%s'", sn.SerialNumber, unmarshaled.SerialNumber)
	}
}

func TestPaginationResult_Initialization(t *testing.T) {
	result := PaginationResult{
		OnuInformationList: []ONUInfoPerBoard{
			{
				Board:        1,
				PON:          8,
				ID:           1,
				Name:         "ONU1",
				OnuType:      "F670",
				SerialNumber: "SN001",
				RXPower:      "-20",
				Status:       "Online",
			},
			{
				Board:        1,
				PON:          8,
				ID:           2,
				Name:         "ONU2",
				OnuType:      "F670",
				SerialNumber: "SN002",
				RXPower:      "-21",
				Status:       "Offline",
			},
		},
		Count: 2,
	}

	if result.Count != 2 {
		t.Errorf("Expected Count 2, got %d", result.Count)
	}

	if len(result.OnuInformationList) != 2 {
		t.Errorf("Expected 2 items, got %d", len(result.OnuInformationList))
	}

	if result.OnuInformationList[0].Name != "ONU1" {
		t.Errorf("Expected first ONU name 'ONU1', got '%s'", result.OnuInformationList[0].Name)
	}
}

func TestONUInfoPerBoard_AllFields(t *testing.T) {
	info := ONUInfoPerBoard{}

	// Test that all fields can be set
	info.Board = 1
	info.PON = 8
	info.ID = 5
	info.Name = "Test"
	info.OnuType = "F670"
	info.SerialNumber = "SN123"
	info.RXPower = "-20"
	info.Status = "Online"

	if info.Board != 1 || info.PON != 8 || info.ID != 5 {
		t.Error("Failed to set integer fields")
	}

	if info.Name != "Test" || info.Status != "Online" {
		t.Error("Failed to set string fields")
	}
}

func TestONUCustomerInfo_AllFields(t *testing.T) {
	info := ONUCustomerInfo{}

	// Set all fields
	info.Board = 1
	info.PON = 8
	info.ID = 5
	info.Name = "Customer"
	info.Description = "Desc"
	info.OnuType = "F670"
	info.SerialNumber = "SN123"
	info.RXPower = "-20"
	info.TXPower = "2"
	info.Status = "Online"
	info.IPAddress = "10.0.0.1"
	info.LastOnline = "2024-01-01"
	info.LastOffline = "2024-01-01"
	info.Uptime = "1 day"
	info.LastDownTimeDuration = "5 min"
	info.LastOfflineReason = "PowerOff"
	info.GponOpticalDistance = "5000"

	// Verify all fields are set
	if info.Description != "Desc" {
		t.Error("Failed to set Description")
	}

	if info.IPAddress != "10.0.0.1" {
		t.Error("Failed to set IPAddress")
	}

	if info.Uptime != "1 day" {
		t.Error("Failed to set Uptime")
	}
}
