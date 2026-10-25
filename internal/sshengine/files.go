package sshengine

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const maxDirectoryBytes = 1 << 20
const maxDirectoryEntries = 2000

var errFiles = errors.New("não foi possível listar: verifique o caminho, as permissões e o suporte SFTP")

type FileEntry struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Modified    int64  `json:"modified"`
	Permissions string `json:"permissions"`
	Directory   bool   `json:"directory"`
	Symlink     bool   `json:"symlink"`
}
type Directory struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

// ListFiles opens a short-lived SFTP channel on the already verified SSH transport.
// A byte budget bounds even directories that exceed the entry limit.
func (m *Manager) ListFiles(id, directory string) (Directory, error) {
	result := Directory{Entries: []FileEntry{}}
	if len(directory) > 4096 || strings.ContainsRune(directory, 0) {
		return result, errFiles
	}
	if directory == "" {
		directory = "."
	}
	s, err := m.get(id)
	if err != nil {
		return result, err
	}
	if !s.filesMu.TryLock() {
		return result, errors.New("uma leitura SFTP já está em andamento")
	}
	defer s.filesMu.Unlock()
	s.mu.Lock()
	client := s.client
	if s.state != "open" || client == nil || s.ctx.Err() != nil {
		s.mu.Unlock()
		return result, ErrClosed
	}
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	s.filesCancel = cancel
	s.mu.Unlock()
	defer func() { cancel(); s.mu.Lock(); s.filesCancel = nil; s.mu.Unlock() }()
	// Opening a channel has no SSH context API. If the peer stalls before it
	// exists, close the transport; once opened, cancel only this SFTP channel.
	var channelMu sync.Mutex
	var remote *ssh.Session
	stop := context.AfterFunc(ctx, func() {
		channelMu.Lock()
		defer channelMu.Unlock()
		if remote != nil {
			_ = remote.Close()
		} else {
			_ = client.Close()
		}
	})
	defer stop()
	opened, err := client.NewSession()
	channelMu.Lock()
	remote = opened
	channelMu.Unlock()
	if err != nil {
		return result, errFiles
	}
	defer opened.Close()
	if ctx.Err() != nil {
		return result, errors.New("leitura SFTP cancelada ou tempo esgotado")
	}
	input, err := opened.StdinPipe()
	if err != nil {
		return result, errFiles
	}
	output, err := opened.StdoutPipe()
	if err != nil {
		return result, errFiles
	}
	// Drain stderr without retaining remote content.
	opened.Stderr = io.Discard
	if err = opened.RequestSubsystem("sftp"); err != nil {
		return result, errFiles
	}
	limited := &io.LimitedReader{R: output, N: maxDirectoryBytes}
	files, err := sftp.NewClientPipe(limited, input, sftp.MaxConcurrentRequestsPerFile(1))
	if err != nil {
		return result, errFiles
	}
	defer func() {
		// Close both channel directions before waiting for the SFTP reader;
		// a peer may ignore stdin EOF while leaving stdout open.
		_ = opened.Close()
		_ = files.Close()
	}()
	resolved, err := files.RealPath(directory)
	if err != nil || len(resolved) > 4096 {
		return result, errFiles
	}
	entries, err := files.ReadDirContext(ctx, resolved)
	if limited.N == 0 || len(entries) > maxDirectoryEntries {
		return result, errors.New("diretório excede o limite de 2.000 itens ou 1 MiB de resposta; abra um subdiretório")
	}
	if ctx.Err() != nil {
		return result, errors.New("leitura SFTP cancelada ou tempo esgotado")
	}
	if err != nil {
		return result, errFiles
	}
	result.Path = resolved
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." || path.Base(name) != name || len(name) > 4096 {
			continue
		}
		result.Entries = append(result.Entries, FileEntry{Name: name, Size: entry.Size(), Modified: entry.ModTime().Unix(), Permissions: entry.Mode().String(), Directory: entry.IsDir(), Symlink: entry.Mode()&os.ModeSymlink != 0})
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		a, b := result.Entries[i], result.Entries[j]
		if a.Directory != b.Directory {
			return a.Directory
		}
		return a.Name < b.Name
	})
	return result, nil
}
func (m *Manager) CancelFiles(id string) {
	s, err := m.get(id)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.filesCancel != nil {
		s.filesCancel()
	}
}
