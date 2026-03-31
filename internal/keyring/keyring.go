// Package keyring provides token persistence via the OS native keychain.
// It uses the service key "com.gitura.app" for all keyring operations.
package keyring

import (
	"github.com/zalando/go-keyring"

	"gitura/internal/logger"
)

const (
	service = "com.gitura.app"
	account = "github-token"
)

// Keyringer is the interface used for OS keychain operations.
// It is satisfied by the real keyring and by test doubles.
type Keyringer interface {
	Set(service, account, password string) error
	Get(service, account string) (string, error)
	Delete(service, account string) error
}

// osKeyring adapts the package-level functions from go-keyring to the
// Keyringer interface so that production code can be swapped for test doubles.
type osKeyring struct{}

// Set stores pwd in the OS keychain under svc and acct.
func (osKeyring) Set(svc, acct, pwd string) error {
	return keyring.Set(svc, acct, pwd)
}

// Get retrieves the password stored in the OS keychain for svc and acct.
func (osKeyring) Get(svc, acct string) (string, error) {
	return keyring.Get(svc, acct)
}

// Delete removes the password stored in the OS keychain for svc and acct.
func (osKeyring) Delete(svc, acct string) error {
	return keyring.Delete(svc, acct)
}

// defaultKeyring is the production keyring used when no override is supplied.
var defaultKeyring Keyringer = osKeyring{}

// SaveToken persists the GitHub access token to the OS keychain.
func SaveToken(token string) error {
	return SaveTokenWith(defaultKeyring, token)
}

// LoadToken retrieves the stored GitHub access token from the OS keychain.
// Returns an error if no token is stored.
func LoadToken() (string, error) {
	return LoadTokenWith(defaultKeyring)
}

// DeleteToken removes the stored GitHub access token from the OS keychain.
// Returns an error only if the delete fails; a missing token is not an error.
func DeleteToken() error {
	return DeleteTokenWith(defaultKeyring)
}

// SaveTokenWith persists the token using the supplied Keyringer.
func SaveTokenWith(k Keyringer, token string) error {
	logger.L.Debug("keyring: saving token")
	err := k.Set(service, account, token)
	if err != nil {
		logger.L.Error("keyring: save failed", "err", err)
	} else {
		logger.L.Debug("keyring: token saved")
	}
	return err
}

// LoadTokenWith retrieves the token using the supplied Keyringer.
func LoadTokenWith(k Keyringer) (string, error) {
	logger.L.Debug("keyring: loading token")
	token, err := k.Get(service, account)
	if err != nil {
		logger.L.Debug("keyring: load failed", "err", err)
	} else {
		logger.L.Debug("keyring: token loaded", "empty", token == "")
	}
	return token, err
}

// DeleteTokenWith removes the token using the supplied Keyringer.
// A missing token is treated as a no-op (not an error).
func DeleteTokenWith(k Keyringer) error {
	logger.L.Debug("keyring: deleting token")
	err := k.Delete(service, account)
	if err == keyring.ErrNotFound {
		logger.L.Debug("keyring: token not found (no-op)")
		return nil
	}
	if err != nil {
		logger.L.Error("keyring: delete failed", "err", err)
	} else {
		logger.L.Debug("keyring: token deleted")
	}
	return err
}
