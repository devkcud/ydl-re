package config

import (
	"crypto/sha256"
	"encoding/hex"
)

func Encode(c Config) ([]byte, string, error) {
	data, err := encodeConfig(c)
	if err != nil {
		return nil, "", err
	}
	return data, contentToken(data), nil
}

func Decode(data []byte) (Config, string, error) {
	c, err := decodeConfig(data)
	if err != nil {
		return Config{}, "", err
	}
	return c, contentToken(data), nil
}

func contentToken(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
