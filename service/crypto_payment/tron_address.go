package crypto_payment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// TRON addresses are base58check over a 21-byte payload: a 0x41 prefix followed
// by the same 20-byte hash EVM chains use. Event topics and log addresses come
// back from the node as raw hex, while everything operator-facing (config, order
// rows, block explorers) is base58 — so a scanner has to convert, and comparing
// the two forms directly is exactly the mistake that kept the TRON scanner from
// ever matching a deposit.
const (
	base58Alphabet    = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	tronAddressPrefix = 0x41
	tronAddressHexLen = 40
)

func base58CheckEncode(payload []byte) string {
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	full := append(append([]byte{}, payload...), second[:4]...)

	number := new(big.Int).SetBytes(full)
	base := big.NewInt(58)
	remainder := new(big.Int)
	encoded := make([]byte, 0, len(full)*2)
	for number.Sign() > 0 {
		number.DivMod(number, base, remainder)
		encoded = append(encoded, base58Alphabet[remainder.Int64()])
	}
	for _, b := range full {
		if b != 0 {
			break
		}
		encoded = append(encoded, base58Alphabet[0])
	}
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(encoded)
}

func base58CheckDecode(encoded string) ([]byte, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, fmt.Errorf("empty base58 address")
	}
	number := new(big.Int)
	base := big.NewInt(58)
	for _, symbol := range trimmed {
		index := strings.IndexRune(base58Alphabet, symbol)
		if index < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", symbol)
		}
		number.Add(number.Mul(number, base), big.NewInt(int64(index)))
	}
	decoded := number.Bytes()
	leadingZeros := 0
	for _, symbol := range trimmed {
		if symbol != rune(base58Alphabet[0]) {
			break
		}
		leadingZeros++
	}
	full := append(make([]byte, leadingZeros), decoded...)
	if len(full) < 5 {
		return nil, fmt.Errorf("base58 address too short")
	}
	payload := full[:len(full)-4]
	checksum := full[len(full)-4:]
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	if !bytes.Equal(second[:4], checksum) {
		return nil, fmt.Errorf("base58 checksum mismatch")
	}
	return payload, nil
}

// tronHexFromAddress turns the base58 form used in config and order rows into the
// bare 20-byte hex a node reports, so scanning can compare hex against hex and
// never has to case-fold base58 (which is case-sensitive: TDrQ and tdrq are
// different addresses, and EqualFold on them would be a silent wallet mix-up).
func tronHexFromAddress(address string) (string, error) {
	payload, err := base58CheckDecode(address)
	if err != nil {
		return "", err
	}
	if len(payload) != 21 || payload[0] != tronAddressPrefix {
		return "", fmt.Errorf("not a TRON address: %q", strings.TrimSpace(address))
	}
	return hex.EncodeToString(payload[1:]), nil
}

// tronAddressFromHex is the inverse, for reporting a counterparty we only ever
// saw as a log topic.
func tronAddressFromHex(hexAddress string) (string, error) {
	normalized := normalizeTronHex(hexAddress)
	if len(normalized) > tronAddressHexLen {
		// An event topic is a 32-byte word with the address right-aligned; a log's
		// own address field is already bare. Taking the low 20 bytes handles both,
		// and also handles a 21-byte form that still carries the 0x41 prefix.
		normalized = normalized[len(normalized)-tronAddressHexLen:]
	}
	if len(normalized) < tronAddressHexLen {
		normalized = strings.Repeat("0", tronAddressHexLen-len(normalized)) + normalized
	}
	raw, err := hex.DecodeString(normalized)
	if err != nil {
		return "", err
	}
	return base58CheckEncode(append([]byte{tronAddressPrefix}, raw...)), nil
}

// tronTopicToHex extracts the bare 20-byte hex address from a 32-byte event topic.
func tronTopicToHex(topic string) string {
	normalized := normalizeTronHex(topic)
	if len(normalized) > tronAddressHexLen {
		return normalized[len(normalized)-tronAddressHexLen:]
	}
	return normalized
}

func normalizeTronHex(value string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "0x"))
}
