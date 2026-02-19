package ssh

import "net"

type AuthMethod interface{}

type PublicKey interface{}

type Signer interface{}

type HostKeyCallback func(hostname string, remote net.Addr, key PublicKey) error

type ClientConfig struct {
	User            string
	Auth            []AuthMethod
	HostKeyCallback HostKeyCallback
}

type Client struct{}

func Password(password string) AuthMethod {
	return password
}

func PublicKeys(signers ...Signer) AuthMethod {
	return signers
}

func ParsePrivateKey(_ []byte) (Signer, error) {
	return struct{}{}, nil
}

func ParsePrivateKeyWithPassphrase(_ []byte, _ []byte) (Signer, error) {
	return struct{}{}, nil
}

func InsecureIgnoreHostKey() HostKeyCallback {
	return func(hostname string, remote net.Addr, key PublicKey) error {
		_ = hostname
		_ = remote
		_ = key
		return nil
	}
}

func Dial(network, address string, config *ClientConfig) (*Client, error) {
	_ = network
	_ = address
	_ = config
	return &Client{}, nil
}

func (c *Client) Close() error {
	return nil
}
