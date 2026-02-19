package sftpsync

import (
	"context"
	"fmt"

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

	if err := syncMod(ctx, client, localDir, remoteDir); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
