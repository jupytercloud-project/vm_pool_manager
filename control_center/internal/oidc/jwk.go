package oidc

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

func jwkToPublicKey(key map[string]interface{}) (*rsa.PublicKey, error) {
	kty, _ := key["kty"].(string)
	if kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", kty)
	}

	nStr, ok := key["n"].(string)
	if !ok {
		return nil, fmt.Errorf("missing n")
	}
	eStr, ok := key["e"].(string)
	if !ok {
		return nil, fmt.Errorf("missing e")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}
