package sshengine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// installKeyCommand accepts only a canonical Ed25519 public key. No path, label,
// user name or shell content from the renderer is interpolated into the script.
func installKeyCommand(public string) (string, error) {
	key, _, options, rest, e := ssh.ParseAuthorizedKey([]byte(public))
	if e != nil || key.Type() != ssh.KeyAlgoED25519 || len(options) != 0 || len(strings.TrimSpace(string(rest))) != 0 {
		return "", errors.New("chave pública inválida")
	}
	canonical := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + " NishiTerm"
	blob := strings.Fields(canonical)[1]
	return fmt.Sprintf(`set -eu
umask 077
[ -n "${HOME:-}" ] && [ -d "$HOME" ]
[ ! -L "$HOME/.ssh" ] || exit 20
mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh"
lock="$HOME/.ssh/.nishiterm-key-install.lock"
mkdir "$lock" || exit 21
trap 'rmdir "$lock"' 0
trap 'exit 130' HUP INT TERM
file="$HOME/.ssh/authorized_keys"
[ ! -L "$file" ] || exit 22
if [ -e "$file" ]; then [ -f "$file" ] || exit 23; else : > "$file"; fi
[ "$(wc -c < "$file")" -le 1048576 ] || exit 24
chmod 600 "$file"
if awk '$1 == "ssh-ed25519" && $2 == "%s" {found=1} END {exit !found}' "$file"; then exit 0; fi
# Preserve restrictions: never add an unrestricted duplicate of an existing key.
if grep -F -q '%s' "$file"; then exit 25; fi
printf '\n%%s\n' '%s' >> "$file"
`, blob, blob, canonical), nil
}

// InstallPublicKey modifies only the current remote user's authorized_keys.
// It never uploads private material and never installs without an existing SSH session.
func (m *Manager) InstallPublicKey(id, host string, port int, user, public string) error {
	command, err := installKeyCommand(public)
	if err != nil {
		return err
	}
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if !s.filesMu.TryLock() {
		return errors.New("aguarde a operação remota em andamento")
	}
	defer s.filesMu.Unlock()
	s.mu.Lock()
	client := s.client
	if s.state != "open" || client == nil || s.ctx.Err() != nil || s.host != host || s.port != port || s.user != user {
		s.mu.Unlock()
		return errors.New("sessão não corresponde ao destino selecionado")
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()
	var mu sync.Mutex
	var remote *ssh.Session
	stop := context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		if remote != nil {
			_ = remote.Close()
		} else {
			_ = client.Close()
		}
	})
	defer stop()
	opened, err := client.NewSession()
	mu.Lock()
	remote = opened
	mu.Unlock()
	if err != nil {
		return errors.New("não foi possível abrir canal de instalação")
	}
	defer opened.Close()
	if ctx.Err() != nil {
		return errors.New("instalação cancelada; verifique o servidor antes de repetir")
	}
	opened.Stdout = io.Discard
	opened.Stderr = io.Discard
	if err = opened.Run(command); err != nil {
		return errors.New("instalação não confirmada: verifique suporte a shell POSIX, permissões e authorized_keys; não houve repetição automática")
	}
	return nil
}
