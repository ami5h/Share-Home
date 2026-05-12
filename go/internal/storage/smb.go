package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"bytes"
	"log"
	"net"
	"path/filepath"
	"time"
	"fmt"

	"github.com/hirochachacha/go-smb2"
)

type SMBStore struct {
	conn    net.Conn
	session *smb2.Session
	fs      *smb2.Share
	baseDir string
	aesGCM  cipher.AEAD
	config  smbConfig
}

type smbConfig struct {
	host, share, username, password string
}

func NewSMBStore(host, share, username, password, baseDir, encryptionKey string) (*SMBStore, error) {
	conn, err := net.DialTimeout("tcp", host+":445", 10*time.Second)
	if err != nil {
		return nil, err
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username,
			Password: password,
		},
	}

	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	mountPath := `\\` + host + `\` + share
	fs, err := session.Mount(mountPath)
	if err != nil {
		session.Logoff()
		conn.Close()
		return nil, err
	}

	if err := fs.MkdirAll(baseDir, 0755); err != nil {
		fs.Umount()
		session.Logoff()
		conn.Close()
		return nil, err
	}

	s := &SMBStore{
		conn:    conn,
		session: session,
		fs:      fs,
		baseDir: baseDir,
		config: smbConfig{
			host:     host,
			share:    share,
			username: username,
			password: password,
		},
	}

	if encryptionKey != "" {
		keyBytes, err := hex.DecodeString(encryptionKey)
		if err != nil {
			// Not valid hex — hash to a uniform 32-byte key
			hash := sha256.Sum256([]byte(encryptionKey))
			keyBytes = hash[:]
		}
		if len(keyBytes) > 32 {
			keyBytes = keyBytes[:32]
		} else if len(keyBytes) < 32 {
			padded := make([]byte, 32)
			copy(padded, keyBytes)
			keyBytes = padded
		}
		block, err := aes.NewCipher(keyBytes)
		if err != nil {
			return nil, err
		}
		s.aesGCM, err = cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *SMBStore) fullPath(key string) string {
	return filepath.Join(s.baseDir, key)
}

func (s *SMBStore) reconnect() error {
	// Clean up stale resources
	s.fs.Umount()
	s.session.Logoff()
	s.conn.Close()

	log.Println("SMB connection dropped, reconnecting...")
	conn, err := net.DialTimeout("tcp", s.config.host+":445", 10*time.Second)
	if err != nil {
		return fmt.Errorf("smb reconnect dial: %w", err)
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     s.config.username,
			Password: s.config.password,
		},
	}

	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smb reconnect dial: %w", err)
	}

	mountPath := `\\` + s.config.host + `\` + s.config.share
	fs, err := session.Mount(mountPath)
	if err != nil {
		session.Logoff()
		conn.Close()
		return fmt.Errorf("smb reconnect mount: %w", err)
	}

	s.conn = conn
	s.session = session
	s.fs = fs

	log.Println("SMB reconnected successfully")
	return nil
}

func (s *SMBStore) Put(key string, body io.Reader) error {
	path := s.fullPath(key)
	s.fs.MkdirAll(filepath.Dir(path), 0755)
	f, err := s.fs.Create(path)
	if err != nil {
		if rerr := s.reconnect(); rerr == nil {
			s.fs.MkdirAll(filepath.Dir(path), 0755)
			f, err = s.fs.Create(path)
		}
		if err != nil {
			return err
		}
	}
	defer f.Close()

	if s.aesGCM != nil {
		nonce := make([]byte, s.aesGCM.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return err
		}
		if _, err := f.Write(nonce); err != nil {
			return err
		}
		data, err := io.ReadAll(body)
		if err != nil {
			return err
		}
		_, err = f.Write(s.aesGCM.Seal(nil, nonce, data, nil))
		return err
	}

	_, err = io.Copy(f, body)
	return err
}

func (s *SMBStore) Get(key string) (io.ReadCloser, int64, error) {
	path := s.fullPath(key)
	f, err := s.fs.Open(path)
	if err != nil {
		if rerr := s.reconnect(); rerr == nil {
			f, err = s.fs.Open(path)
		}
		if err != nil {
			return nil, 0, err
		}
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}

	if s.aesGCM != nil {
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, 0, err
		}
		ns := s.aesGCM.NonceSize()
		if len(data) < ns {
			return nil, 0, io.ErrUnexpectedEOF
		}
		decrypted, err := s.aesGCM.Open(nil, data[:ns], data[ns:], nil)
		if err != nil {
			return nil, 0, err
		}
		return io.NopCloser(bytes.NewReader(decrypted)), int64(len(decrypted)), nil
	}

	return f, info.Size(), nil
}

func (s *SMBStore) Delete(key string) error {
	err := s.fs.Remove(s.fullPath(key))
	if err != nil {
		if rerr := s.reconnect(); rerr == nil {
			err = s.fs.Remove(s.fullPath(key))
		}
	}
	return err
}

func (s *SMBStore) SpaceInfo() (total, used, free int64, err error) {
	info, err := s.fs.Statfs(s.baseDir)
	if err != nil {
		return 0, 0, 0, err
	}
	clusterSize := int64(info.BlockSize()) * int64(info.FragmentSize())
	total = clusterSize * int64(info.TotalBlockCount())
	free = clusterSize * int64(info.AvailableBlockCount())
	used = total - free
	if used < 0 {
		used = 0
	}
	return total, used, free, nil
}

func (s *SMBStore) Close() error {
	s.fs.Umount()
	s.session.Logoff()
	return s.conn.Close()
}
