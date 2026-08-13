package gatewayauth

import (
	"encoding/base64"
	"encoding/json"
)

func EncodeUserPayloadForTest(claim any) (string, error) {
	raw, err := json.Marshal(claim)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
