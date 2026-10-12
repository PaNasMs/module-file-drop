package drop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ACLzz/qshare"
	"github.com/ACLzz/qshare/qserver"
	"github.com/PaNasMs/module-file-drop/internal/access"
	"github.com/PaNasMs/module-sdk/auth"
)

const windowDuration = 10 * time.Minute
const offerDuration = 60 * time.Second

type Offer struct {
	ID      string            `json:"id"`
	Files   []qshare.FileMeta `json:"files"`
	PIN     uint16            `json:"pin"`
	Expires time.Time         `json:"expires"`
	answer  chan bool
}

type Transfer struct {
	ID       string            `json:"id"`
	Files    []qshare.FileMeta `json:"files"`
	Started  time.Time         `json:"started"`
	Finished time.Time         `json:"finished,omitempty"`
	Status   string            `json:"status"`
	Error    string            `json:"error,omitempty"`
	owner    access.Owner
	inbox    string
	writers  int
	seen     int
	ended    bool
	failed   bool
}

type Window struct {
	Owner   access.Owner `json:"owner"`
	Expires time.Time    `json:"expires"`
}

type State struct {
	Window   *Window    `json:"window,omitempty"`
	Busy     bool       `json:"busy"`
	Offer    *Offer     `json:"offer,omitempty"`
	Transfer *Transfer  `json:"transfer,omitempty"`
	History  []Transfer `json:"history"`
	Inbox    string     `json:"inbox"`
	Default  bool       `json:"defaultInbox"`
}

type receiver interface {
	Listen() error
	Stop() error
}

type Service struct {
	mu       sync.Mutex
	root     string
	allowed  map[string]bool
	window   *Window
	offer    *Offer
	transfer *Transfer
	history  []Transfer
	receiver receiver
	timer    *time.Timer
	write    func(access.Account, string, qshare.FilePayload) error
}

func New(root string, allowed map[string]bool) *Service {
	return &Service{root: root, allowed: allowed, write: writePayload}
}

func (s *Service) Active() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.window != nil || s.transfer != nil {
		return 1
	}
	return 0
}

func account(id auth.Identity) (access.Account, error) {
	u, err := user.Lookup(id.Username)
	if err != nil {
		return access.Account{}, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil || uid != id.UID {
		return access.Account{}, access.ErrDenied
	}
	a := access.Account{Username: id.Username, UID: id.UID, Principal: id.Principal, Home: u.HomeDir, Admin: id.Role == "admin", Enabled: true}
	if !a.Valid() {
		return access.Account{}, access.ErrDenied
	}
	return a, nil
}

func key(owner access.Owner) string {
	return fmt.Sprintf("%d-%x-%x", owner.UID, []byte(owner.Username), []byte(owner.Principal))
}

func (s *Service) inbox(a access.Account) (string, bool, error) {
	owner, _ := a.Owner()
	raw, err := os.ReadFile(filepath.Join(s.root, key(owner)+".json"))
	if os.IsNotExist(err) {
		path, e := a.DefaultInbox()
		return path, true, e
	}
	if err != nil {
		return "", false, err
	}
	var setting struct {
		Owner access.Owner `json:"owner"`
		Path  string       `json:"path"`
	}
	if json.Unmarshal(raw, &setting) != nil || !a.Owns(setting.Owner) || !filepath.IsAbs(setting.Path) {
		return "", false, access.ErrDenied
	}
	return setting.Path, false, nil
}

func (s *Service) SetInbox(a access.Account, path string) error {
	if !a.Valid() {
		return access.ErrDenied
	}
	if path != "" {
		rel, err := filepath.Rel(a.Home, path)
		if err != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("choose a folder inside your home")
		}
	}
	owner, _ := a.Owner()
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}
	file := filepath.Join(s.root, key(owner)+".json")
	if path == "" {
		err := os.Remove(file)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	raw, _ := json.Marshal(struct {
		Owner access.Owner `json:"owner"`
		Path  string       `json:"path"`
	}{owner, path})
	tmp, err := os.CreateTemp(s.root, ".inbox-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(raw)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

func (s *Service) State(a access.Account) (State, error) {
	inbox, def, err := s.inbox(a)
	if err != nil {
		return State{}, err
	}
	state := State{Inbox: inbox, Default: def, History: []Transfer{}}
	s.mu.Lock()
	defer s.mu.Unlock()
	state.Busy = s.window != nil && !a.Owns(s.window.Owner)
	if s.window != nil && a.Owns(s.window.Owner) {
		w := *s.window
		state.Window = &w
		if s.offer != nil {
			o := *s.offer
			o.answer = nil
			state.Offer = &o
		}
	}
	if s.transfer != nil && a.Owns(s.transfer.owner) {
		t := *s.transfer
		state.Transfer = &t
	}
	for _, item := range s.history {
		if a.Owns(item.owner) {
			state.History = append(state.History, item)
		}
	}
	return state, nil
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Service) Start(a access.Account) error {
	owner, err := a.Owner()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.window != nil || s.transfer != nil {
		return errors.New("receiver is busy")
	}
	server, err := qserver.NewBuilder().WithHostname("PaNasMs").Build(s.onOffer, nil, s.onFile, s.onEnd)
	if err != nil {
		return err
	}
	if err = server.Listen(); err != nil {
		return err
	}
	s.receiver = server
	s.window = &Window{Owner: owner, Expires: time.Now().Add(windowDuration)}
	s.timer = time.AfterFunc(windowDuration, s.expire)
	return nil
}

func (s *Service) expire() {
	s.mu.Lock()
	if s.window == nil || s.transfer != nil {
		s.mu.Unlock()
		return
	}
	receiver := s.closeWindowLocked()
	s.mu.Unlock()
	if receiver != nil {
		_ = receiver.Stop()
	}
}

func (s *Service) closeWindowLocked() receiver {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.offer != nil {
		select {
		case s.offer.answer <- false:
		default:
		}
		s.offer = nil
	}
	s.window = nil
	receiver := s.receiver
	s.receiver = nil
	return receiver
}

func (s *Service) Stop(a access.Account) error {
	s.mu.Lock()
	if s.window == nil || !a.Owns(s.window.Owner) || s.transfer != nil {
		s.mu.Unlock()
		return access.ErrDenied
	}
	receiver := s.closeWindowLocked()
	s.mu.Unlock()
	if receiver != nil {
		return receiver.Stop()
	}
	return nil
}

func validFiles(files []qshare.FileMeta) bool {
	if len(files) == 0 || len(files) > 128 {
		return false
	}
	for _, f := range files {
		if !validName(f.Name) || f.Size < 0 || f.Size > 100<<30 {
			return false
		}
	}
	return true
}

func (s *Service) onOffer(text *qshare.TextMeta, files []qshare.FileMeta, pin uint16) bool {
	if text != nil || !validFiles(files) {
		return false
	}
	s.mu.Lock()
	if s.window == nil || s.offer != nil || s.transfer != nil || time.Now().After(s.window.Expires) {
		s.mu.Unlock()
		return false
	}
	offer := &Offer{ID: newID(), Files: append([]qshare.FileMeta(nil), files...), PIN: pin, Expires: time.Now().Add(offerDuration), answer: make(chan bool, 1)}
	s.offer = offer
	s.mu.Unlock()
	timer := time.NewTimer(offerDuration)
	defer timer.Stop()
	select {
	case approved := <-offer.answer:
		return approved
	case <-timer.C:
		s.mu.Lock()
		if s.offer == offer {
			s.offer = nil
		}
		s.mu.Unlock()
		return false
	}
}

func (s *Service) Decide(a access.Account, id string, approve bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.window == nil || !a.Owns(s.window.Owner) || s.offer == nil || s.offer.ID != id || time.Now().After(s.offer.Expires) {
		return access.ErrDenied
	}
	if approve {
		inbox, _, err := s.inbox(a)
		if err != nil {
			return err
		}
		s.transfer = &Transfer{ID: s.offer.ID, Files: s.offer.Files, Started: time.Now(), Status: "receiving", owner: s.window.Owner, inbox: inbox}
	}
	s.offer.answer <- approve
	s.offer = nil
	return nil
}

func (s *Service) onFile(payload qshare.FilePayload) {
	s.mu.Lock()
	transfer := s.transfer
	if transfer == nil || transfer.seen >= len(transfer.Files) {
		s.mu.Unlock()
		_ = payload.Pr.Close()
		return
	}
	transfer.seen++
	transfer.writers++
	owner := transfer.owner
	inbox := transfer.inbox
	s.mu.Unlock()
	id, err := auth.LookupPanel(owner.Username, s.allowed)
	var a access.Account
	if err == nil {
		a, err = account(id)
	}
	if err == nil && !a.Owns(owner) {
		err = access.ErrDenied
	}
	if err == nil {
		err = s.write(a, inbox, payload)
	}
	_ = payload.Pr.Close()
	s.mu.Lock()
	transfer.writers--
	if err != nil {
		transfer.failed = true
		transfer.Error = err.Error()
	}
	receiver := s.finishLocked()
	s.mu.Unlock()
	if receiver != nil {
		_ = receiver.Stop()
	}
}

func (s *Service) onEnd(complete bool) {
	s.mu.Lock()
	if s.transfer != nil {
		s.transfer.ended = true
		if !complete || s.transfer.seen != len(s.transfer.Files) {
			s.transfer.failed = true
			if s.transfer.Error == "" {
				s.transfer.Error = "sender disconnected before completing the transfer"
			}
		}
	}
	receiver := s.finishLocked()
	s.mu.Unlock()
	if receiver != nil {
		go receiver.Stop()
	}
}

func (s *Service) finishLocked() receiver {
	if s.transfer == nil || !s.transfer.ended || s.transfer.writers != 0 {
		return nil
	}
	t := *s.transfer
	t.Finished = time.Now()
	if t.failed {
		t.Status = "failed"
	} else {
		t.Status = "completed"
	}
	s.history = append([]Transfer{t}, s.history...)
	if len(s.history) > 30 {
		s.history = s.history[:30]
	}
	s.transfer = nil
	return s.closeWindowLocked()
}
