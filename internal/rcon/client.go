package rcon

import (
	"fmt"

	battleye "github.com/multiplay/go-battleye"
)

type Client struct {
	address  string
	password string
}

func New(address, password string) *Client {
	return &Client{address: address, password: password}
}

func (c *Client) Say(message string) error {
	cli, err := battleye.Dial(c.address, c.password)
	if err != nil {
		return fmt.Errorf("connect battleye: %w", err)
	}
	defer cli.Close()
	if _, err := cli.Command("say -1 " + message); err != nil {
		return fmt.Errorf("send battleye command: %w", err)
	}
	return nil
}
