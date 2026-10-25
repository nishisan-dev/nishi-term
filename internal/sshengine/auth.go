package sshengine

import (
	"bytes"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

const maxPrivateKey = 1 << 20

var errPrivateKey = errors.New("não foi possível abrir a chave: confira o arquivo, formato e passphrase")

// Keys remain backend-only. No fallback to passwords or agents is implicit.
func authentication(c Config) (ssh.AuthMethod, error) {
	if c.AuthMethod == "" || c.AuthMethod == "password" {
		return ssh.Password(c.Password), nil
	}
	info, err := os.Stat(c.PrivateKeyPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPrivateKey {
		return nil, errPrivateKey
	}
	f, err := os.Open(c.PrivateKeyPath)
	if err != nil {
		return nil, errPrivateKey
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPrivateKey+1))
	defer clear(data)
	if err != nil || len(data) > maxPrivateKey || !boundedKeyWork(data) {
		return nil, errPrivateKey
	}
	var signer ssh.Signer
	if c.Passphrase == "" {
		signer, err = ssh.ParsePrivateKey(data)
	} else {
		pass := []byte(c.Passphrase)
		signer, err = ssh.ParsePrivateKeyWithPassphrase(data, pass)
		clear(pass)
	}
	if err != nil {
		return nil, errPrivateKey
	}
	return ssh.PublicKeys(signer), nil
}

// A bounded file size alone does not bound the work factor of an encrypted key.
// OpenSSH defaults to 16 bcrypt rounds. Accept up to 64; malformed files fail closed.
func boundedKeyWork(data []byte) bool {
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	defer clear(block.Bytes)
	if block.Type != "OPENSSH PRIVATE KEY" {
		return true
	}
	magic := []byte("openssh-key-v1\x00")
	if !bytes.HasPrefix(block.Bytes, magic) {
		return false
	}
	rest := block.Bytes[len(magic):]
	take := func() ([]byte, bool) {
		if len(rest) < 4 {
			return nil, false
		}
		n := uint64(binary.BigEndian.Uint32(rest))
		rest = rest[4:]
		if n > uint64(len(rest)) {
			return nil, false
		}
		out := rest[:int(n)]
		rest = rest[int(n):]
		return out, true
	}
	if _, ok := take(); !ok {
		return false
	}
	kdf, ok := take()
	if !ok {
		return false
	}
	opts, ok := take()
	if !ok {
		return false
	}
	if string(kdf) == "none" {
		return len(opts) == 0
	}
	if string(kdf) != "bcrypt" {
		return false
	}
	rest = opts
	salt, ok := take()
	if !ok || len(salt) > 1024 || len(rest) != 4 {
		return false
	}
	rounds := binary.BigEndian.Uint32(rest)
	return rounds > 0 && rounds <= 64
}
