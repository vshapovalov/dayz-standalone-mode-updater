package sftp

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Client struct{}

func NewClient(conn *ssh.Client) (*Client, error) {
	_ = conn
	return &Client{}, nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) MkdirAll(path string) error {
	_ = path
	return nil
}

func (c *Client) Create(path string) (io.WriteCloser, error) {
	_ = path
	return nopWriteCloser{}, nil
}

func (c *Client) Open(path string) (io.ReadCloser, error) {
	_ = path
	return io.NopCloser(bytes.NewReader(nil)), nil
}

type nopWriteCloser struct{}

func (n nopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (n nopWriteCloser) Close() error {
	return nil
}

func Join(elem ...string) string {
	return filepath.ToSlash(filepath.Join(elem...))
}

func Clean(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
