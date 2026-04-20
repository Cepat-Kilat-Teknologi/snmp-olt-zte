package model

import "time"

// OltConfig struct is a struct that represents the OLT configuration
type OltConfig struct {
	BaseOID                   string // Base OID for the OLT
	OnuIDNameOID              string // OID for the ONU ID Name
	OnuTypeOID                string // OID for the ONU Type
	OnuSerialNumberOID        string // OID for the ONU Serial Number
	OnuRxPowerOID             string // OID for the ONU RX Power
	OnuTxPowerOID             string // OID for the ONU TX Power
	OnuStatusOID              string // OID for the ONU Status
	OnuIPAddressOID           string // OID for the ONU IP Address
	OnuDescriptionOID         string // OID for the ONU Description
	OnuLastOnlineOID          string // OID for the ONU Last Online Time
	OnuLastOfflineOID         string // OID for the ONU Last Offline Time
	OnuLastOfflineReasonOID   string // OID for the ONU Last Offline Reason
	OnuGponOpticalDistanceOID string // OID for the ONU GPON Optical Distance
}

// ONUInfo struct is a struct that represent the ONU information
type ONUInfo struct {
	ID   string `json:"onu_id"` // The unique identifier for the ONU
	Name string `json:"name"`   // The name of the ONU
}

// ONUInfoPerBoard struct is a struct that represents the ONU information per board
type ONUInfoPerBoard struct {
	Board        int    `json:"board"`         // The board ID where the ONU is located
	PON          int    `json:"pon"`           // The PON ID where the ONU is connected
	ID           int    `json:"onu_id"`        // The ID of the ONU
	Name         string `json:"name"`          // The name of the ONU
	OnuType      string `json:"onu_type"`      // The type of the ONU
	SerialNumber string `json:"serial_number"` // The serial number of the ONU
	RXPower      string `json:"rx_power"`      // The receiving power of the ONU
	Status       string `json:"status"`        // The current status of the ONU
}

// ONUCustomerInfo struct is a struct that represents the detailed ONU information for the customer
type ONUCustomerInfo struct {
	Board                int    `json:"board"`                   // The board ID
	PON                  int    `json:"pon"`                     // The PON ID
	ID                   int    `json:"onu_id"`                  // The ONU ID
	Name                 string `json:"name"`                    // The name of the ONU
	Description          string `json:"description"`             // Description of the ONU
	OnuType              string `json:"onu_type"`                // Type of the ONU
	SerialNumber         string `json:"serial_number"`           // Serial number of the ONU
	RXPower              string `json:"rx_power"`                // RX power level
	TXPower              string `json:"tx_power"`                // TX power level
	Status               string `json:"status"`                  // Operational status
	IPAddress            string `json:"ip_address"`              // IP Address assigned to the ONU
	LastOnline           string `json:"last_online"`             // Timestamp of last online status
	LastOffline          string `json:"last_offline"`            // Timestamp of the last offline status
	Uptime               string `json:"uptime"`                  // Duration of uptime
	LastDownTimeDuration string `json:"last_down_time_duration"` // Duration of last downtime
	LastOfflineReason    string `json:"offline_reason"`          // Reason for last offline event
	GponOpticalDistance  string `json:"gpon_optical_distance"`   // Optical distance to the ONU
}

// OnuID struct is a struct that represent the ONU ID
type OnuID struct {
	Board int `json:"board"`  // Board ID
	PON   int `json:"pon"`    // PON ID
	ID    int `json:"onu_id"` // ONU ID
}

// OnuOnlyID struct is a struct that represent only the ONU ID without board and PON
type OnuOnlyID struct {
	ID int `json:"onu_id"` // ONU ID
}

// OnuSerialNumber struct is a struct that represents the ONU serial number
type OnuSerialNumber struct {
	Board        int    `json:"board"`         // Board ID
	PON          int    `json:"pon"`           // PON ID
	ID           int    `json:"onu_id"`        // ONU ID
	SerialNumber string `json:"serial_number"` // Serial Number
}

// PaginationResult struct is a struct that represents the pagination result
type PaginationResult struct {
	OnuInformationList []ONUInfoPerBoard // List of ONU information for the current page
	Count              int               // Total count of items
}

// TrapEvent represents a processed SNMP trap event from the OLT.
// Enriched with ONU identity info (name, alamat, serial, type) for notification context.
type TrapEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Source       string    `json:"source"`
	Board        int       `json:"board"`
	PON          int       `json:"pon"`
	OnuID        int       `json:"onu_id"`
	EventType    string    `json:"event_type"`
	Status       string    `json:"status"`
	Name         string    `json:"name,omitempty"`
	Description  string    `json:"description,omitempty"`
	OnuType      string    `json:"onu_type,omitempty"`
	SerialNumber string    `json:"serial_number,omitempty"`
	RXPower      string    `json:"rx_power,omitempty"`
	LastOffline  string    `json:"last_offline,omitempty"`
	LastOnline   string    `json:"last_online,omitempty"`
}
