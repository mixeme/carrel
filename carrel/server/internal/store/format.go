package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"carrel.dev/carrel/server/internal/crypt"
)

const (
	StateFile     = "state.enc"
	magic         = "CRL1"
	FormatVersion = 1
	tmpSuffix     = ".tmp"
)

var (
	ErrNewerVersion = errors.New("store: state file from a newer build")
	ErrCorruptFile  = errors.New("store: corrupt state file")
	ErrShortFile    = errors.New("store: state file too short")
)

func encodeFile(version uint32, serverKey [crypt.KeyLen]byte, state State) ([]byte, error) {
	plain, err := jsonMarshal(state)
	if err != nil {
		return nil, err
	}
	box, err := crypt.EncryptState(serverKey, plain, version)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := buf.WriteString(magic); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, version); err != nil {
		return nil, err
	}
	if _, err := buf.Write(box); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeFile(data []byte, serverKey [crypt.KeyLen]byte) (uint32, State, error) {
	if len(data) < len(magic)+4 {
		return 0, emptyState(), ErrShortFile
	}
	if string(data[:len(magic)]) != magic {
		return 0, emptyState(), ErrCorruptFile
	}
	version := binary.BigEndian.Uint32(data[len(magic):])
	if version > FormatVersion {
		return version, emptyState(), ErrNewerVersion
	}
	plain, err := crypt.DecryptState(serverKey, data[len(magic)+4:], version)
	if err != nil {
		return version, emptyState(), fmt.Errorf("%w: %v", ErrCorruptFile, err)
	}
	var st State
	if err := jsonUnmarshal(plain, &st); err != nil {
		return version, emptyState(), fmt.Errorf("%w: %v", ErrCorruptFile, err)
	}
	return version, st, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := path + tmpSuffix
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmp)
	}()

	if _, err := io.Copy(f, bytes.NewReader(data)); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDir(dir)
}

// syncDir makes the rename itself durable: without it a crash can leave the
// directory entry pointing at neither the old file nor the new one. Best
// effort — Windows does not let a directory handle be synced, and there the
// rename is already the strongest guarantee the filesystem offers, so a
// failure here is not reported as a failed write.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer d.Close()
	_ = d.Sync()
	return nil
}

// jsonMarshal/jsonUnmarshal are thin wrappers so tests can stay in the package
// without importing encoding/json in every file.
func jsonMarshal(v any) ([]byte, error) {
	return jsonMarshalImpl(v)
}

func jsonUnmarshal(b []byte, v any) error {
	return jsonUnmarshalImpl(b, v)
}
