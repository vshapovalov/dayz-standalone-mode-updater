package rcon

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	battleye "github.com/multiplay/go-battleye"
)

type dialFn func(address, password string) (commandClient, error)

type commandClient interface {
	Command(command string) (string, error)
	Close() error
}

type Controller struct {
	cfg  config.Config
	dial dialFn
	logf func(format string, args ...any)
}

func NewController(cfg config.Config) *Controller {
	return &Controller{
		cfg:  cfg,
		dial: dialBattleye,
		logf: func(string, ...any) {},
	}
}

func (c *Controller) WithLogger(logf func(format string, args ...any)) *Controller {
	if logf != nil {
		c.logf = logf
	}
	return c
}

func (c *Controller) Tick(ctx context.Context, now time.Time, st *state.State) {
	if st == nil {
		return
	}
	for _, serverCfg := range c.cfg.Servers {
		serverState, ok := st.Servers[serverCfg.ID]
		if !ok || !serverState.NeedsShutdown {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}

		address := fmt.Sprintf("%s:%d", serverCfg.RCON.Host, serverCfg.RCON.Port)
		client, err := c.dial(address, serverCfg.RCON.Password)
		if err != nil {
			c.logf("rcon connect failed for server %s: %v", serverCfg.ID, err)
			st.Servers[serverCfg.ID] = serverState
			continue
		}

		if serverState.ShutdownDeadlineAt != nil && now.Before(*serverState.ShutdownDeadlineAt) {
			if shouldAnnounce(now, serverState.NextAnnounceAt) {
				remaining := RemainingMinutes(*serverState.ShutdownDeadlineAt, now)
				message := FormatMessage(c.cfg.Shutdown.MessageTemplate, remaining)
				if err := exec(client, sayCommand(message)); err != nil {
					c.logf("rcon announce failed for server %s: %v", serverCfg.ID, err)
				} else {
					next := now.Add(time.Duration(c.cfg.Shutdown.AnnounceEverySeconds) * time.Second)
					serverState.NextAnnounceAt = &next
				}
			}
		} else {
			if err := exec(client, sayCommand(c.cfg.Shutdown.FinalMessage)); err != nil {
				c.logf("rcon final message failed for server %s: %v", serverCfg.ID, err)
			}
			if err := exec(client, "#shutdown"); err != nil {
				c.logf("rcon shutdown failed for server %s: %v", serverCfg.ID, err)
			} else {
				serverState.NeedsShutdown = false
				serverState.Stage = state.StageIdle
				n := now.UTC()
				serverState.ShutdownSentAt = &n
			}
		}

		if err := client.Close(); err != nil {
			c.logf("rcon close failed for server %s: %v", serverCfg.ID, err)
		}
		st.Servers[serverCfg.ID] = serverState
	}
}

func RemainingMinutes(deadline, now time.Time) int {
	if !deadline.After(now) {
		return 0
	}
	remainingSeconds := deadline.Sub(now).Seconds()
	return int(math.Ceil(remainingSeconds / 60.0))
}

func FormatMessage(template string, minutes int) string {
	return strings.ReplaceAll(template, "{minutes}", strconv.Itoa(minutes))
}

func shouldAnnounce(now time.Time, next *time.Time) bool {
	return next == nil || !now.Before(*next)
}

func sayCommand(message string) string {
	return "say -1 " + message
}

func exec(client commandClient, command string) error {
	if _, err := client.Command(command); err != nil {
		return err
	}
	return nil
}

func dialBattleye(address, password string) (commandClient, error) {
	return battleye.Dial(address, password)
}
