package drop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ACLzz/qshare"
	"github.com/PaNasMs/module-file-drop/internal/access"
)

func TestOfferRequiresOwnerDecision(t *testing.T) {
	owner := access.Account{Username: "alice", UID: 1000, Principal: "principal-a", Home: "/home/alice", Enabled: true}
	bob := access.Account{Username: "bob", UID: 1001, Principal: "principal-b", Home: "/home/bob", Enabled: true}
	s := New(t.TempDir(), nil)
	s.window = &Window{Owner: access.Owner{Username: "alice", UID: 1000, Principal: "principal-a"}, Expires: time.Now().Add(time.Minute)}
	if s.Active() != 1 {
		t.Fatal("open receiving window was not counted as active")
	}
	otherState, err := s.State(bob)
	if err != nil || !otherState.Busy || otherState.Window != nil {
		t.Fatalf("another user was not shown that the receiver is busy: %+v %v", otherState, err)
	}
	result := make(chan bool, 1)
	go func() { result <- s.onOffer(nil, []qshare.FileMeta{{Name: "photo.jpg", Size: 12}}, 1234) }()
	var offer *Offer
	for i := 0; i < 100; i++ {
		s.mu.Lock()
		offer = s.offer
		s.mu.Unlock()
		if offer != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if offer == nil {
		t.Fatal("offer not shown")
	}
	if err := s.Decide(bob, offer.ID, true); err == nil {
		t.Fatal("another user approved the offer")
	}
	if err := s.Decide(owner, offer.ID, true); err != nil {
		t.Fatal(err)
	}
	if !<-result {
		t.Fatal("owner approval was lost")
	}
	s.onEnd(false)
	if s.Active() != 0 {
		t.Fatal("finished receiving window remained active")
	}
	state, err := s.State(owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.History) != 1 || state.History[0].Status != "failed" {
		t.Fatal(state.History)
	}
	other, err := s.State(bob)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.History) != 0 {
		t.Fatal("another user can see the transfer")
	}
}

func TestInboxSettingsSeparateLinuxIdentities(t *testing.T) {
	legacy := access.Owner{Username: "alice", UID: 1000}
	if key(legacy) == key(access.Owner{Username: "other", UID: 1000}) ||
		key(legacy) == key(access.Owner{Username: "alice", UID: 1001}) ||
		key(legacy) == key(access.Owner{Username: "alice", UID: 1000, Principal: "new"}) {
		t.Fatal("inbox settings key reused across identities")
	}
}

func TestInboxStaysInsideHome(t *testing.T) {
	home := t.TempDir()
	s := New(t.TempDir(), nil)
	a := access.Account{Username: "alice", UID: 1000, Home: home, Enabled: true}
	for _, path := range []string{filepath.Dir(home), filepath.Join(home, "..", "other"), "/tmp"} {
		if err := s.SetInbox(a, path); err == nil {
			t.Fatalf("accepted destination outside home: %q", path)
		}
	}
	inside := filepath.Join(home, "Photos")
	if err := s.SetInbox(a, inside); err != nil {
		t.Fatal(err)
	}
	if got, _, err := s.inbox(a); err != nil || got != inside {
		t.Fatalf("stored inbox %q: %v", got, err)
	}
}

func TestRejectUnsafeFileMetadata(t *testing.T) {
	for _, name := range []string{"", "../file", "a/b", "a\\b", "a\x00b", strings.Repeat("a", 201)} {
		if validFiles([]qshare.FileMeta{{Name: name, Size: 1}}) {
			t.Fatalf("accepted %q", name)
		}
	}
	if validFiles([]qshare.FileMeta{{Name: "ok", Size: -1}}) {
		t.Fatal("accepted negative size")
	}
}

func TestWriterPreservesConflictsAndRemovesPartialFiles(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "Quick Share")
	old := os.Stdin
	defer func() { os.Stdin = old }()
	write := func(body string, size int64) error {
		input, err := os.CreateTemp(t.TempDir(), "input")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(input.Name())
		if _, err = input.WriteString(body); err != nil {
			t.Fatal(err)
		}
		if _, err = input.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		os.Stdin = input
		defer input.Close()
		return WriteFromStdin(inbox, "photo.jpg", size)
	}
	if err := write("first", 5); err != nil {
		t.Fatal(err)
	}
	if err := write("second", 6); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"photo.jpg": "first", "photo (1).jpg": "second"} {
		data, err := os.ReadFile(filepath.Join(inbox, name))
		if err != nil || string(data) != content {
			t.Fatalf("%s: %q %v", name, data, err)
		}
	}
	if err := write("short", 10); err == nil {
		t.Fatal("accepted truncated file")
	}
	if _, err := os.Stat(filepath.Join(inbox, "photo (2).jpg")); !os.IsNotExist(err) {
		t.Fatalf("partial file left behind: %v", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(inbox, link); err != nil {
		t.Fatal(err)
	}
	if _, err := openFolder(link); err == nil {
		t.Fatal("followed directory symlink")
	}
}
