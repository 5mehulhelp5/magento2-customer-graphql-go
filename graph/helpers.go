package graph

import (
	"encoding/base64"
	"strconv"
)

// decodeUID decodes a base64-encoded UID to an integer ID.
func decodeUID(uid string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(uid)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(decoded))
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func coalesce(values ...*string) *string {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}
