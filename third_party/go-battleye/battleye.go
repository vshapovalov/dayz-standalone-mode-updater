package battleye

type Client struct{}

func Dial(address, password string) (*Client, error) {
	_ = address
	_ = password
	return &Client{}, nil
}

func (c *Client) Command(command string) (string, error) {
	_ = command
	return "", nil
}

func (c *Client) Close() error {
	return nil
}
