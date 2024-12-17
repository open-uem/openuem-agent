package vnc

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/hex"
	"log"
)

// Ref: https://stackoverflow.com/questions/41579325/golang-how-do-i-decrypt-with-des-cbc-and-pkcs7
func DESEncode(password string) []byte {
	cipherKey := "e84ad660c4721ae0"
	bCipherKey, err := hex.DecodeString(cipherKey)
	if err != nil {
		log.Fatalf("could not decode cipher key hex string")
	}

	block, err := des.NewCipher(bCipherKey)
	if err != nil {
		log.Fatalf("could not create cipher block")
	}

	bIV, err := hex.DecodeString("0000000000000000")
	if err != nil {
		log.Fatalf("could not decode IV hex string")
	}

	// If password length is less than 8 characters we must padd byte(0) to the left
	padding := 8 - len(password)
	padtext := bytes.Repeat([]byte{byte(0)}, padding)

	encryptedPassword := make([]byte, 8)
	blockMode := cipher.NewCBCEncrypter(block, bIV)
	blockMode.CryptBlocks(encryptedPassword, append([]byte(password), padtext...))

	return encryptedPassword
}

func UltraVNCEncrypt(pin string) string {
	var ultraVNCDESKey = []byte{0xE8, 0x4A, 0xD6, 0x60, 0xC4, 0x72, 0x1A, 0xE0}
	// Pad the password with zeroes, then take the first 8 bytes.
	pin = pin + "\x00\x00\x00\x00\x00\x00\x00\x00"
	pin = pin[:8]
	// Create a DES cipher using the same key as UltraVNC.
	block, err := des.NewCipher(ultraVNCDESKey)
	if err != nil {
		return ""
	}
	// Encrypt password.
	encryptedPassword := make([]byte, block.BlockSize())
	block.Encrypt(encryptedPassword, []byte(pin))
	// Append an arbitrary byte as per UltraVNC's algorithm.
	encryptedPassword = append(encryptedPassword, 0)
	// Return encrypted password as a hex-encoded string.
	return hex.EncodeToString(encryptedPassword)
}
