package utils

import "strings"

func GetVendorUserID(userType, userID string) string {
	return strings.ToLower(userType) + "_" + userID
}

func ParseVendorUserID(vendorID string) (userType, userID string) {
	parts := strings.SplitN(vendorID, "_", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", vendorID
}
