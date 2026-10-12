package access

import "testing"

func TestDeviceVisibilityAndInboxOwnership(t *testing.T) {
	owner := Owner{Username: "alice", UID: 1000, Principal: "original-account"}
	cases := []struct {
		name         string
		account      Account
		view, change bool
	}{
		{"owner", Account{Username: "alice", UID: 1000, Principal: "original-account", Enabled: true}, true, true},
		{"other user", Account{Username: "member", UID: 1001, Principal: "member-account", Enabled: true}, false, false},
		{"administrator", Account{Username: "admin", UID: 1002, Principal: "admin-account", Enabled: true, Admin: true}, true, false},
		{"recreated account", Account{Username: "alice", UID: 1000, Principal: "new-account", Enabled: true}, false, false},
		{"disabled administrator", Account{Username: "admin", UID: 1002, Principal: "admin-account", Admin: true}, false, false},
		{"legacy user cannot own principal-bound device", Account{Username: "alice", UID: 1000, Enabled: true}, false, false},
		{"root", Account{Username: "root", Principal: "root", Enabled: true, Admin: true}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.account.CanViewDevice(owner); got != tc.view {
				t.Fatalf("view: got %v, want %v", got, tc.view)
			}
			if got := tc.account.CanChangeInbox(owner); got != tc.change {
				t.Fatalf("change inbox: got %v, want %v", got, tc.change)
			}
		})
	}
}

func TestLegacyLinuxAccountCanOwnItsReceivingWindow(t *testing.T) {
	owner := Account{Username: "alice", UID: 1000, Enabled: true, Home: "/home/alice"}
	stored, err := owner.Owner()
	if err != nil || !owner.Owns(stored) {
		t.Fatalf("legacy account cannot own its receiver: %v", err)
	}
	if got, err := owner.DefaultInbox(); err != nil || got != "/home/alice/Quick Share" {
		t.Fatalf("legacy default inbox: %q, %v", got, err)
	}
	for _, other := range []Account{
		{Username: "other", UID: 1000, Enabled: true},
		{Username: "alice", UID: 1001, Enabled: true},
		{Username: "alice", UID: 1000, Principal: "new-principal", Enabled: true},
	} {
		if other.Owns(stored) {
			t.Fatalf("different identity inherited receiver: %+v", other)
		}
	}
}

func TestDefaultInboxTracksActualHomeWithoutCreatingIt(t *testing.T) {
	a := Account{Username: "alice", UID: 1000, Principal: "account", Enabled: true, Home: "/srv/data/home/alice"}
	got, err := a.DefaultInbox()
	if err != nil || got != "/srv/data/home/alice/Quick Share" {
		t.Fatalf("got %q, %v", got, err)
	}
	a.Home = "/srv/new/home/alice"
	got, err = a.DefaultInbox()
	if err != nil || got != "/srv/new/home/alice/Quick Share" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, home := range []string{"", ".", "/", "/a/.."} {
		a.Home = home
		if _, err := a.DefaultInbox(); err == nil {
			t.Fatalf("accepted invalid home %q", home)
		}
	}
}
