package spot

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"
)

func MakeRandomSenderID() string {
	id := fmt.Sprintf("%d", time.Now().UnixMilli())
	return id[len(id)-8:]
}

func makeSendingTime() string {
	return time.Now().UTC().Format("20060102-15:04:05.000")
}

func calCheckSum(txt string) string {
	var cks int64
	for idx := 0; idx < len(txt); idx++ {
		cks += ord(txt[idx : idx+1])
	}
	var sum = cks % 256
	if sum < 10 {
		return fmt.Sprintf("00%d", sum)
	} else if sum < 100 {
		return fmt.Sprintf("0%d", sum)
	} else {
		return fmt.Sprintf("%d", sum)
	}
}

func ord(txt string) int64 {
	code := int64(txt[0])

	// High surrogate (could change last hex to 0xDB7F to treat high private surrogates as single characters)
	if 0xd800 <= code && code <= 0xdbff {
		hi := code
		if len(txt) == 1 {
			return code
			// we could also throw an error as it is not a complete character, but someone may want to know
		}
		var low = int64(txt[1])
		return (hi-0xd800)*0x400 + (low - 0xdc00) + 0x10000
	}

	// Low surrogate
	if 0xdc00 <= code && code <= 0xdfff {
		return code
		// we could also throw an error as it is not a complete character, but someone may want to know
	}
	return code
}

func parsePrivateKey(apiSecret string) (crypto.PrivateKey, error) {
	block, _ := pem.Decode([]byte(apiSecret))
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func signPayload(payload string, privateKey crypto.PrivateKey) (string, error) {
	key, ok := privateKey.(crypto.Signer)
	if !ok {
		return "", fmt.Errorf("key does not implement crypto.Signer")
	}

	signature, err := key.Sign(nil, []byte(payload), crypto.Hash(0))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}
