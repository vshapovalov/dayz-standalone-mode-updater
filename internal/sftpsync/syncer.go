package sftpsync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Syncer struct {
	address  string
	username string
	password string
}

func New(address, username, password string) *Syncer {
	return &Syncer{address: address, username: username, password: password}
}

func (s *Syncer) SyncDirectory(ctx context.Context, localDir, remoteDir string) error {
	conn, err := ssh.Dial("tcp", s.address, &ssh.ClientConfig{
		User:            s.username,
		Auth:            []ssh.AuthMethod{ssh.Password(s.password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return fmt.Errorf("dial sftp ssh: %w", err)
	}
	defer conn.Close()
	client, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("create sftp client: %w", err)
	}
	defer client.Close()

	if err := client.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("mkdir remote root: %w", err)
	}
	return filepath.Walk(localDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		target := filepath.ToSlash(filepath.Join(remoteDir, rel))
		if info.IsDir() {
			return client.MkdirAll(target)
		}
		return uploadFile(client, path, target)
	})
}

func uploadFile(client *sftp.Client, localPath, remotePath string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer src.Close()
	dst, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote file: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	return nil
}
