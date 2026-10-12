package drop

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/ACLzz/qshare"
	"github.com/PaNasMs/module-file-drop/internal/access"
	"github.com/PaNasMs/module-sdk/maintenance"
	"github.com/PaNasMs/module-sdk/userfiles"
)

func validName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 200 || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, c := range name {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

func writePayload(a access.Account, inbox string, payload qshare.FilePayload) error {
	release, err := maintenance.Acquire()
	if err != nil {
		return err
	}
	defer release()
	if !validName(payload.Meta.Name) || payload.Meta.Size < 0 || payload.Meta.Size > 100<<30 || !filepath.IsAbs(inbox) {
		return errors.New("invalid file metadata")
	}
	u, err := user.Lookup(a.Username)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil || uid != a.UID {
		return access.ErrDenied
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	groups, err := u.GroupIds()
	if err != nil {
		return err
	}
	ids := make([]uint32, 0, len(groups))
	for _, value := range groups {
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		ids = append(ids, uint32(n))
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, "write", inbox, payload.Meta.Name, strconv.FormatInt(payload.Meta.Size, 10))
	cmd.Stdin = payload.Pr
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + u.HomeDir, "LANG=C.UTF-8"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: ids}}
	if err = cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("receive file: %s", message)
		}
		return fmt.Errorf("receive file: %w", err)
	}
	return nil
}

func openFolder(path string) (int, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return -1, errors.New("invalid destination")
	}
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			_ = syscall.Close(fd)
			return -1, errors.New("invalid destination")
		}
		flags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
		next, e := syscall.Openat(fd, part, flags, 0)
		if e == syscall.ENOENT && i == len(parts)-1 {
			if e = syscall.Mkdirat(fd, part, 0777); e == nil {
				next, e = syscall.Openat(fd, part, flags, 0)
			}
		}
		_ = syscall.Close(fd)
		if e != nil {
			return -1, e
		}
		fd = next
	}
	return fd, nil
}

func uniqueName(name string, n int) string {
	if n == 0 {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s (%d)%s", base, n, ext)
}

func WriteFromStdin(inbox, name string, size int64) (err error) {
	if !validName(name) || size < 0 || size > 100<<30 {
		return errors.New("invalid file metadata")
	}
	mask, err := userfiles.DefaultUmask()
	if err != nil {
		return err
	}
	syscall.Umask(mask)
	dir, err := openFolder(inbox)
	if err != nil {
		return err
	}
	defer syscall.Close(dir)
	var file *os.File
	var chosen string
	for n := 0; n < 10000; n++ {
		chosen = uniqueName(name, n)
		if len(chosen) > 255 {
			return errors.New("file name is too long")
		}
		fd, e := syscall.Openat(dir, chosen, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0666)
		if e == syscall.EEXIST {
			continue
		}
		if e != nil {
			return e
		}
		file = os.NewFile(uintptr(fd), chosen)
		break
	}
	if file == nil {
		return errors.New("no available file name")
	}
	defer func() {
		if err != nil {
			_ = syscall.Unlinkat(dir, chosen)
		}
	}()
	defer file.Close()
	written, err := io.CopyN(file, os.Stdin, size)
	if err != nil || written != size {
		return errors.New("transfer ended before the announced size")
	}
	var extra [1]byte
	n, readErr := os.Stdin.Read(extra[:])
	if n != 0 || readErr != io.EOF {
		return errors.New("transfer exceeded the announced size")
	}
	return file.Sync()
}
