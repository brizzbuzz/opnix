package secrets

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/brizzbuzz/opnix/internal/config"
)

type preparedTemplate struct {
	path     string
	content  []byte
	mode     os.FileMode
	uid, gid int
}

func metadata(owner, group, mode string) (os.FileMode, int, int, error) {
	if mode == "" {
		mode = "0600"
	}
	m, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid output mode")
	}
	uid, gid := -1, -1
	if owner == "root" {
		uid = 0
	} else if owner != "" {
		u, err := user.Lookup(owner)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unknown output owner %q", owner)
		}
		uid, err = strconv.Atoi(u.Uid)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid owner ID")
		}
	}
	if group == "root" {
		gid = 0
	} else if group != "" {
		g, err := user.LookupGroup(group)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unknown output group %q", group)
		}
		gid, err = strconv.Atoi(g.Gid)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid group ID")
		}
	}
	return os.FileMode(m), uid, gid, nil
}

// canonicalDestination follows existing parent symlinks but not the final component.
func canonicalDestination(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(abs)
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Join(resolved, filepath.Base(abs)), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", err
		}
		missing = append(missing, filepath.Base(parent))
		parent = next
	}
}

func (p *Processor) prepare(cfg *config.Config) ([]preparedTemplate, error) {
	var paths []string
	add := func(path string) error {
		if strings.Contains(path, "..") {
			return fmt.Errorf("unsafe output path")
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		canonical, err := canonicalDestination(abs)
		if err != nil {
			return err
		}
		for _, candidate := range []string{abs, canonical} {
			for _, prefix := range []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin", "/boot", "/dev", "/proc", "/sys", "/etc/passwd", "/etc/shadow", "/etc/group"} {
				if strings.HasPrefix(candidate, prefix) {
					return fmt.Errorf("unsafe output path %q", candidate)
				}
				resolvedPrefix, err := canonicalDestination(prefix)
				if err == nil && strings.HasPrefix(candidate, resolvedPrefix) {
					return fmt.Errorf("unsafe output path %q", candidate)
				}
				if resolvedPrefix, err := filepath.EvalSymlinks(prefix); err == nil && strings.HasPrefix(candidate, resolvedPrefix) {
					return fmt.Errorf("unsafe output path %q", candidate)
				}
			}
		}
		for _, previous := range paths {
			if canonical == previous || strings.HasPrefix(canonical, previous+string(os.PathSeparator)) || strings.HasPrefix(previous, canonical+string(os.PathSeparator)) {
				return fmt.Errorf("conflicting output paths")
			}
		}
		paths = append(paths, canonical)
		return nil
	}
	for i, s := range cfg.Secrets {
		path, err := p.resolveSecretPathWithTemplate(s, fmt.Sprintf("secret[%d]", i))
		if err != nil {
			return nil, err
		}
		if err := add(path); err != nil {
			return nil, err
		}
		// The legacy writer follows a final symlink. Include its target in the
		// collision set without changing how legacy secrets are written.
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil, fmt.Errorf("cannot resolve secret destination symlink: %w", err)
			}
			if err := add(target); err != nil {
				return nil, err
			}
		} else if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if _, _, _, err := metadata(s.Owner, s.Group, s.Mode); err != nil {
			return nil, err
		}
		for _, link := range s.Symlinks {
			if err := add(link); err != nil {
				return nil, err
			}
		}
	}
	result := make([]preparedTemplate, len(cfg.Templates))
	for i, t := range cfg.Templates {
		path := p.resolveSecretPath(t.Path, t.Name)
		if err := add(path); err != nil {
			return nil, err
		}
		if err := rejectSymlink(path); err != nil {
			return nil, err
		}
		mode, uid, gid, err := metadata(t.Owner, t.Group, t.Mode)
		if err != nil {
			return nil, err
		}
		if mode&07000 != 0 {
			return nil, fmt.Errorf("template mode cannot contain special permission bits")
		}
		result[i] = preparedTemplate{path: path, mode: mode, uid: uid, gid: gid}
	}
	return result, nil
}

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("template destination is a symlink")
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("template destination is not a regular file")
	}
	return nil
}

func writeTemplate(t preparedTemplate) error {
	if err := rejectSymlink(t.path); err != nil {
		return err
	}
	info, err := os.Lstat(t.path)
	if err == nil {
		old, err := os.ReadFile(t.path)
		if err != nil {
			return err
		}
		if bytes.Equal(old, t.content) {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("cannot inspect output ownership")
			}
			uid, gid := -1, -1
			if t.uid != -1 && int(stat.Uid) != t.uid {
				uid = t.uid
			}
			if t.gid != -1 && int(stat.Gid) != t.gid {
				gid = t.gid
			}
			if uid != -1 || gid != -1 {
				if err := os.Chown(t.path, uid, gid); err != nil {
					return err
				}
			}
			const permissionBits = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
			if info.Mode()&permissionBits != t.mode&permissionBits || uid != -1 || gid != -1 {
				return os.Chmod(t.path, t.mode)
			}
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(t.path), ".opnix-template-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if _, err := f.Write(t.content); err != nil {
		return err
	}
	if t.uid != -1 || t.gid != -1 {
		if err := f.Chown(t.uid, t.gid); err != nil {
			return err
		}
	}
	if err := f.Chmod(t.mode); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := rejectSymlink(t.path); err != nil {
		return err
	}
	return os.Rename(f.Name(), t.path)
}
