package ssh

import "net"

type AuthMethod interface{}

type PublicKey interface{}

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
