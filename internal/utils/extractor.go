package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// ExtractONUID function is used to extract ONU ID from OID string
// It assumes the ONU ID is the last numeric component of the dot-separated OID string.
func ExtractONUID(oid string) string {
	// Split the OID name and take the last component
	parts := strings.Split(oid, ".")
	if len(parts) > 0 {
		// Check if the last component is a valid number
		lastComponent := parts[len(parts)-1]
		if _, err := strconv.Atoi(lastComponent); err == nil {
			return lastComponent // Return if valid integer string
		}
	}
	return "" // Return an empty string if the OID is invalid or empty (default value)
}

// ExtractIDOnuID function is used to extract ONU ID from OID any
// Validates the type is string and performs extraction.
func ExtractIDOnuID(oid any) int {
	if oid == nil {
		return 0 // Return 0 if input is nil
	}

	switch v := oid.(type) {
	case string:
		parts := strings.Split(v, ".") // Split string by dot
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			id, err := strconv.Atoi(lastPart) // Convert the last part to int
			if err == nil {
				return id // Return ID
			}
		}
		return 0
	default:
		return 0 // Return 0 for other types
	}
}

// ExtractName function is used to extract name from OID value
// Handling both string and byte slice types.
func ExtractName(oidValue any) string {
	switch v := oidValue.(type) {
	case string:
		// Data is string, return it
		return v
	case []byte:
		// Data is a byte slice, convert to string
		return string(v)
	default:
		// Data type is not recognized
		return "Unknown" // Return "Unknown" default
	}
}

// ExtractSerialNumber function is used to extract serial number from OID value
// Removes "1," prefix if present (common in some ZTE OLTs).
func ExtractSerialNumber(oidValue any) string {
	switch v := oidValue.(type) {
	case string:
		// If the string starts with "1,", remove it from the string
		if strings.HasPrefix(v, "1,") {
			return v[2:]
		}
		return v
	case []byte:
		// Convert byte slice to string
		strValue := string(v)
		if strings.HasPrefix(strValue, "1,") {
			return strValue[2:] // Remove prefix
		}
		return strValue
	default:
		// Data type is not recognized
		return "" // Return empty string
	}
}

// ConvertAndMultiply function is used to convert the PDU value to string after multiplying by 0.002 and subtracting 30
// Typically used for converting Optical Power values.
func ConvertAndMultiply(pduValue any) (string, error) {
	// Type asserts pduValue to an integer type
	intValue, ok := pduValue.(int)
	if !ok {
		return "", fmt.Errorf("value is not an integer") // Error if not int
	}

	// Multiply the integer by 0.002 (scale factor)
	result := float64(intValue) * 0.002

	// Subtract 30 (offset)
	result -= 30.0

	// Convert the result to a string with two decimal places
	resultStr := strconv.FormatFloat(result, 'f', 2, 64)

	return resultStr, nil
}

// ExtractAndGetStatus function is used to extract and get status from OID value
// Maps integer status codes to human-readable strings.
func ExtractAndGetStatus(oidValue any) string {
	// Check if oidValue is an integer
	intValue, ok := oidValue.(int)
	if !ok {
		return "Unknown"
	}

	switch intValue {
	case 1:
		return "Logging"
	case 2:
		return "LOS" // Loss of Signal
	case 3:
		return "Synchronization"
	case 4:
		return "Online"
	case 5:
		return "Dying Gasp" // Power failure indication
	case 6:
		return "Auth Failed"
	case 7:
		return "Offline"
	default:
		return "Unknown"
	}
}

// ExtractLastOfflineReason function is used to extract the last offline reason from OID value
// Maps integer reason codes to human-readable strings.
func ExtractLastOfflineReason(oidValue any) string {
	// Check if oidValue is an integer
	intValue, ok := oidValue.(int)
	if !ok {
		return "Unknown"
	}

	switch intValue {
	case 1:
		return "Unknown"
	case 2:
		return "LOS" // Loss of Signal
	case 3:
		return "LOSi"
	case 4:
		return "LOFi" // Loss of Frame
	case 5:
		return "sfi"
	case 6:
		return "loai"
	case 7:
		return "loami"
	case 8:
		return "AuthFail"
	case 9:
		return "PowerOff"
	case 10:
		return "deactiveSucc"
	case 11:
		return "deactiveFail"
	case 12:
		return "Reboot"
	case 13:
		return "Shutdown"
	default:
		return "Unknown"
	}
}

// ExtractGponOpticalDistance function is used to extract GPON optical distance from OID value
func ExtractGponOpticalDistance(oidValue any) string {
	// Check if oidValue is an integer
	intValue, ok := oidValue.(int)
	if !ok {
		return "Unknown"
	}

	return strconv.Itoa(intValue) // Convert integer to string
}
