package access

import (
	"errors"
	"path/filepath"
)

var ErrDenied = errors.New("access denied")

type Account struct {
	Username  string
	UID       int
	Principal string
	Home      string
	Admin     bool
	Enabled   bool
}

type Owner struct {
	Username  string `json:"username"`
	UID       int    `json:"uid"`
	Principal string `json:"principal"`
}

func (a Account) Valid() bool {
	return a.Enabled && a.UID > 0 && a.UID != 65534 && a.Username != ""
}

func (a Account) Owns(owner Owner) bool {
	return a.Valid() && a.Username == owner.Username && a.UID == owner.UID && a.Principal == owner.Principal
}

func (a Account) Owner() (Owner, error) {
	if !a.Valid() {
		return Owner{}, ErrDenied
	}
	return Owner{Username: a.Username, UID: a.UID, Principal: a.Principal}, nil
}

func (a Account) CanViewDevice(owner Owner) bool {
	return a.Valid() && (a.Admin || a.Owns(owner))
}

func (a Account) CanChangeInbox(owner Owner) bool {
	return a.Owns(owner)
}

func (a Account) DefaultInbox() (string, error) {
	if !a.Valid() || !filepath.IsAbs(a.Home) || filepath.Clean(a.Home) == "/" {
		return "", ErrDenied
	}
	return filepath.Join(a.Home, "Quick Share"), nil
}
