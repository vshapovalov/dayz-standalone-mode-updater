package sftp

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client struct{}

type Walker struct{}

type File struct {
	bytes.Buffer
}

func NewClient(conn *ssh.Client) (*Client, error) {
	_ = conn
	return &Client{}, nil
}

func (c *Client) Close() error { return nil }

func (c *Client) MkdirAll(path string) error { _ = path; return nil }

func (c *Client) Create(path string) (io.WriteCloser, error) { _ = path; return nopWriteCloser{}, nil }

func (c *Client) Open(path string) (io.ReadCloser, error) {
	_ = path
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (c *Client) Remove(path string) error { _ = path; return nil }

func (c *Client) RemoveDirectory(path string) error { _ = path; return nil }

func (c *Client) Rename(oldname, newname string) error { _, _ = oldname, newname; return nil }

func (c *Client) Chtimes(path string, atime, mtime time.Time) error {
	_, _, _ = path, atime, mtime
	return nil
}

func (c *Client) Stat(path string) (os.FileInfo, error) { _ = path; return fakeInfo{}, nil }

func (c *Client) Walk(root string) *Walker { _ = root; return &Walker{} }

func (w *Walker) Step() bool { return false }

func (w *Walker) Err() error { return nil }

func (w *Walker) Stat() os.FileInfo { return fakeInfo{} }

func (w *Walker) Path() string { return "" }

type nopWriteCloser struct{}

func (n nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }

func (n nopWriteCloser) Close() error { return nil }

func Join(elem ...string) string { return filepath.ToSlash(filepath.Join(elem...)) }

func Clean(path string) string { return strings.ReplaceAll(path, "\\", "/") }

type fakeInfo struct{}

func (fakeInfo) Name() string       { return "" }
func (fakeInfo) Size() int64        { return 0 }
func (fakeInfo) Mode() fs.FileMode  { return 0 }
func (fakeInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fakeInfo) IsDir() bool        { return false }
func (fakeInfo) Sys() any           { return nil }
