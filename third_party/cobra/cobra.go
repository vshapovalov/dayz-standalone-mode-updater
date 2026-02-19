package cobra

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

type Command struct {
	Use   string
	Short string
	RunE  func(cmd *Command, args []string) error

	children []*Command
	flags    *flag.FlagSet
}

func (c *Command) AddCommand(children ...*Command) {
	c.children = append(c.children, children...)
}

func (c *Command) Execute() error {
	args := os.Args[1:]
	return c.execute(args)
}

func (c *Command) execute(args []string) error {
	if len(args) > 0 {
		for _, child := range c.children {
			if child.commandName() == args[0] {
				return child.run(args[1:])
			}
		}
	}
	if c.RunE != nil {
		return c.run(args)
	}
	return fmt.Errorf("unknown command: %v", args)
}

func (c *Command) run(args []string) error {
	if c.flags != nil {
		if err := c.flags.Parse(args); err != nil {
			return err
		}
		args = c.flags.Args()
	}
	if c.RunE == nil {
		return errors.New("no command action defined")
	}
	return c.RunE(c, args)
}

func (c *Command) commandName() string {
	for i := range c.Use {
		if c.Use[i] == ' ' {
			return c.Use[:i]
		}
	}
	return c.Use
}

type FlagSet struct{ inner *flag.FlagSet }

func (c *Command) Flags() *FlagSet {
	if c.flags == nil {
		c.flags = flag.NewFlagSet(c.commandName(), flag.ContinueOnError)
	}
	return &FlagSet{inner: c.flags}
}

func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	f.inner.StringVar(p, name, value, usage)
}
