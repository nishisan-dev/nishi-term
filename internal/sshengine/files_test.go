package sshengine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFilesBrowseAndTerminalIsolation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	f := serverWithFiles(t, nil, 0, root)
	m := manager(t)
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	listing, err := m.ListFiles(id, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 2 || listing.Entries[0].Name != "folder" || !listing.Entries[0].Directory || listing.Entries[1].Size != 5 {
		t.Fatalf("bad listing: %+v", listing)
	}
	empty, err := m.ListFiles(id, listing.Path+"/folder")
	if err != nil || len(empty.Entries) != 0 {
		t.Fatalf("nested: %+v %v", empty, err)
	}
	if _, err = m.ListFiles(id, listing.Path+"/missing"); err == nil {
		t.Fatal("missing directory accepted")
	}
	var ack uint64
	if err := echoOnce(m, id, &ack, "terminal-after-sftp"); err != nil {
		t.Fatal(err)
	}
	m.Close(id)
	if _, err = m.ListFiles(id, "."); err == nil {
		t.Fatal("closed session accepted")
	}
}
func TestFilesUnsupportedAndCancel(t *testing.T) {
	for _, root := range []string{"", "stall"} {
		t.Run(fmt.Sprintf("root-%s", root), func(t *testing.T) {
			f := serverWithFiles(t, nil, 0, root)
			m := manager(t)
			id := start(t, m, f.config)
			s, _ := m.get(id)
			eventually(t, func() bool { state, _ := s.status(); return state == "open" })
			finished := make(chan error, 1)
			go func() { _, err := m.ListFiles(id, "."); finished <- err }()
			if root == "stall" {
				eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.filesCancel != nil })
				if _, err := m.ListFiles(id, "."); err == nil {
					t.Fatal("parallel listing accepted")
				}
				// Wait for the actual request, not a timing assumption.
				eventually(t, func() bool { return f.sftpStarted.Load() })
				m.CancelFiles(id)
			}
			select {
			case err := <-finished:
				if err == nil {
					t.Fatal("expected failure")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("SFTP stuck")
			}
			var ack uint64
			if err := echoOnce(m, id, &ack, "still-responsive"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestFilesDirectoryLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < maxDirectoryEntries+1; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%04d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := serverWithFiles(t, nil, 0, root)
	m := manager(t)
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	if _, err := m.ListFiles(id, "."); err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("expected bounded listing: %v", err)
	}
	var ack uint64
	if err := echoOnce(m, id, &ack, "after-limit"); err != nil {
		t.Fatal(err)
	}
}
