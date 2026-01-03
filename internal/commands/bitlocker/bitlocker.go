//go:build windows

package bitlocker

import (
	"log"

	bl "github.com/open-uem/glazier/go/bitlocker"
)

func EncryptLocalDisk(driveLetter string) error {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Encrypt the volume
	return volume.Encrypt(bl.XtsAES256, bl.EncryptDataOnly)
}

func DecryptDisk(driveLetter string) error {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Decrypt the volume
	return volume.Decrypt()
}

func SuspendProtection(driveLetter string) error {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Disable Key Protectors
	return volume.DisableKeyProtectors(0)
}

func ResumeProtection(driveLetter string) error {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Resume Key Protectors
	return volume.EnableKeyProtectors()
}

func EncryptRemovableDisk(driveLetter string, passphrase string) (string, error) {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return "", err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Prepare the volume
	if err := volume.Prepare(bl.VolumeTypeDefault, bl.EncryptionTypeUnspecified); err != nil {
		return "", err
	}

	// 3. Protect with passphrase
	volumeKeyProtectorID, err := volume.ProtectWithPassphrase(passphrase)
	if err != nil {
		return "", err
	}

	// 4.Encrypt the volume
	if err := volume.Encrypt(bl.XtsAES256, bl.EncryptDataOnly); err != nil {
		return "", err
	}

	return volumeKeyProtectorID, nil
}

func ChangePassphrase(driveLetter string, volumeKeyProtectorID string, newPassphrase string) (string, error) {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return "", err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Disable key protectors
	if err := volume.DisableKeyProtectors(0); err != nil {
		return "", err
	}

	defer func() {
		if err := volume.EnableKeyProtectors(); err != nil {
			log.Printf("[ERROR]: %v", err)
		}
	}()

	// 3. Change passphrase
	return volume.ChangePassphrase(volumeKeyProtectorID, newPassphrase)
}

func EnableAutoUnlock(driveLetter string, volumeKeyProtectorID string) error {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Enable AutoUnlock
	return volume.EnableAutoUnlock(volumeKeyProtectorID)
}

func DisableAutoUnlock(driveLetter string, volumeKeyProtectorID string) error {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Disable AutoUnlock
	return volume.DisableAutoUnlock()
}

func GetConversionStatus(driveLetter string) (*bl.ConversionStatus, error) {
	// 1. Connect to the volume
	volume, err := bl.Connect(driveLetter)
	if err != nil {
		return nil, err
	}

	defer func() {
		volume.Close()
	}()

	// 2. Get conversion status
	return volume.GetConversionStatus(0)
}
