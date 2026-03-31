package keyring_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"gitura/internal/keyring"
)

// fakeKeyring is an in-memory Keyringer for testing.
type fakeKeyring struct {
	data map[string]string
	err  error // if set, every operation returns this error
}

func newFakeKeyring() *fakeKeyring {
	return &fakeKeyring{data: make(map[string]string)}
}

func (f *fakeKeyring) Set(svc, acct, pwd string) error {
	if f.err != nil {
		return f.err
	}
	f.data[svc+":"+acct] = pwd
	return nil
}

func (f *fakeKeyring) Get(svc, acct string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	val, ok := f.data[svc+":"+acct]
	if !ok {
		return "", gokeyring.ErrNotFound
	}
	return val, nil
}

func (f *fakeKeyring) Delete(svc, acct string) error {
	if f.err != nil {
		return f.err
	}
	key := svc + ":" + acct
	if _, ok := f.data[key]; !ok {
		return gokeyring.ErrNotFound
	}
	delete(f.data, key)
	return nil
}

func TestSaveToken_Success(t *testing.T) {
	k := newFakeKeyring()
	require.NoError(t, keyring.SaveTokenWith(k, "tok123"))
	val, err := keyring.LoadTokenWith(k)
	require.NoError(t, err)
	assert.Equal(t, "tok123", val)
}

func TestSaveToken_Error(t *testing.T) {
	k := &fakeKeyring{data: make(map[string]string), err: errors.New("disk full")}
	err := keyring.SaveTokenWith(k, "tok")
	require.Error(t, err)
}

func TestLoadToken_NotFound(t *testing.T) {
	k := newFakeKeyring()
	_, err := keyring.LoadTokenWith(k)
	require.Error(t, err)
	assert.ErrorIs(t, err, gokeyring.ErrNotFound)
}

func TestDeleteToken_Success(t *testing.T) {
	k := newFakeKeyring()
	require.NoError(t, keyring.SaveTokenWith(k, "tok"))
	require.NoError(t, keyring.DeleteTokenWith(k))
	_, err := keyring.LoadTokenWith(k)
	require.Error(t, err)
}

func TestDeleteToken_NotFound_NoError(t *testing.T) {
	k := newFakeKeyring()
	// delete on empty keyring should be a no-op
	require.NoError(t, keyring.DeleteTokenWith(k))
}

func TestDeleteToken_Error(t *testing.T) {
	k := newFakeKeyring()
	require.NoError(t, keyring.SaveTokenWith(k, "tok"))
	k.err = errors.New("permission denied")
	err := keyring.DeleteTokenWith(k)
	require.Error(t, err)
}
