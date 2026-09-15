package lan

import (
	"encoding/base64"
	"strings"
)

func encodeID(ip string) string {
	return "ipp_" + base64.RawURLEncoding.EncodeToString([]byte("l:"+ip))
}

func decodeID(id string) (string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "ipp_"))
	if err != nil {
		return "", false
	}

	if len(decoded) < 3 || decoded[1] != ':' {
		return "", false
	}

	if decoded[0] != 'l' {
		return "", false
	}

	return string(decoded[2:]), true
}
