package app

import (
	"strings"

	"github.com/AmooVPN/hub/internal/security"
)

func decryptPanelCredentials(encryptedPassword, encryptedAPIToken, secret string) (string, string, error) {
	password, err := decryptPanelSecretValue(encryptedPassword, secret)
	if err != nil {
		return "", "", err
	}
	apiToken, err := decryptPanelSecretValue(encryptedAPIToken, secret)
	if err != nil {
		return "", "", err
	}
	return password, apiToken, nil
}

func decryptPanelSecretValue(encrypted, secret string) (string, error) {
	if strings.TrimSpace(encrypted) == "" {
		return "", nil
	}
	return security.Decrypt(encrypted, secret)
}
