package modlist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Provider interface {
	Load() ([]config.ModConfig, error)
}

type StaticProvider struct {
	mods []config.ModConfig
}

type ParsedMod struct {
	DisplayName string
	Link        string
	WorkshopID  string
	FolderSlug  string
}

type PollResult struct {
	Mods       []ParsedMod
	SortedIDs  []string
	ModsetHash string
	CachePath  string
}

var (
	modRowPattern      = regexp.MustCompile(`(?is)<tr[^>]*data-type\s*=\s*["']ModContainer["'][^>]*>(.*?)</tr>`)
	displayNamePattern = regexp.MustCompile(`(?is)<td[^>]*data-type\s*=\s*["']DisplayName["'][^>]*>(.*?)</td>`)
	linkPattern        = regexp.MustCompile(`(?is)<a[^>]*data-type\s*=\s*["']Link["'][^>]*href\s*=\s*["']([^"']+)["'][^>]*>`)
	tagPattern         = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern       = regexp.MustCompile(`\s+`)
	nonSlugPattern     = regexp.MustCompile(`[^a-z0-9-]`)
	dashPattern        = regexp.MustCompile(`-+`)
	digitsPattern      = regexp.MustCompile(`^[0-9]+$`)
)

func NewStaticProvider(mods []config.ModConfig) *StaticProvider {
	return &StaticProvider{mods: mods}
}

func (p *StaticProvider) Load() ([]config.ModConfig, error) {
	return p.mods, nil
}

func PollServerModlist(ctx context.Context, srv config.ServerConfig, localCacheRoot string, warnf func(string, ...any)) (PollResult, error) {
	html, cachePath, err := fetchModlistHTML(ctx, srv, localCacheRoot)
	if err != nil {
		return PollResult{}, err
	}

	mods := ParseHTMLModlist(html, warnf)
	ids := make([]string, 0, len(mods))
	for _, mod := range mods {
		ids = append(ids, mod.WorkshopID)
	}
	sort.Strings(ids)

	return PollResult{
		Mods:       mods,
		SortedIDs:  ids,
		ModsetHash: HashModset(ids),
		CachePath:  cachePath,
	}, nil
}

func fetchModlistHTML(ctx context.Context, srv config.ServerConfig, localCacheRoot string) (string, string, error) {
	var lastErr error
	for attempt := 1; attempt <= srv.SFTP.MaxRetries; attempt++ {
		html, cachePath, err := fetchModlistHTMLOnce(ctx, srv, localCacheRoot)
		if err == nil {
			return html, cachePath, nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || attempt == srv.SFTP.MaxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(time.Duration(srv.SFTP.RetryBackoffMillis*attempt) * time.Millisecond):
		}
	}
	return "", "", lastErr
}

func fetchModlistHTMLOnce(ctx context.Context, srv config.ServerConfig, localCacheRoot string) (string, string, error) {
	opCtx, cancel := context.WithTimeout(ctx, time.Duration(srv.SFTP.OperationTimeoutSeconds)*time.Second)
	defer cancel()

	address := fmt.Sprintf("%s:%d", srv.SFTP.Host, srv.SFTP.Port)
	sshConfig, err := sshConfigFromServer(srv)
	if err != nil {
		return "", "", err
	}
	connCh := make(chan struct {
		conn *ssh.Client
		err  error
	}, 1)
	go func() {
		conn, err := ssh.Dial("tcp", address, sshConfig)
		connCh <- struct {
			conn *ssh.Client
			err  error
		}{conn: conn, err: err}
	}()
	var conn *ssh.Client
	select {
	case <-opCtx.Done():
		return "", "", opCtx.Err()
	case res := <-connCh:
		if res.err != nil {
			return "", "", fmt.Errorf("dial sftp ssh for server %q: %w", srv.ID, res.err)
		}
		conn = res.conn
	}
	defer conn.Close()
	client, err := sftp.NewClient(conn)
	if err != nil {
		return "", "", fmt.Errorf("create sftp client for server %q: %w", srv.ID, err)
	}
	defer client.Close()

	remotePath := srv.SFTP.RemoteModlistPath
	if strings.TrimSpace(remotePath) == "" {
		remotePath = "/modlist.html"
	}
	type result struct {
		content []byte
		err     error
	}
	resCh := make(chan result, 1)
	go func() {
		remoteFile, err := client.Open(remotePath)
		if err != nil {
			resCh <- result{err: fmt.Errorf("open remote modlist for server %q: %w", srv.ID, err)}
			return
		}
		defer remoteFile.Close()
		content, err := io.ReadAll(remoteFile)
		if err != nil {
			resCh <- result{err: fmt.Errorf("read remote modlist for server %q: %w", srv.ID, err)}
			return
		}
		resCh <- result{content: content}
	}()

	select {
	case <-opCtx.Done():
		return "", "", opCtx.Err()
	case res := <-resCh:
		if res.err != nil {
			return "", "", res.err
		}
		cachePath := filepath.Join(localCacheRoot, "servers", srv.ID, "modlist.html")
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return "", "", fmt.Errorf("create modlist cache dir for server %q: %w", srv.ID, err)
		}
		if err := os.WriteFile(cachePath, res.content, 0o644); err != nil {
			return "", "", fmt.Errorf("write cached modlist for server %q: %w", srv.ID, err)
		}
		return string(res.content), cachePath, nil
	}
}

func sshConfigFromServer(srv config.ServerConfig) (*ssh.ClientConfig, error) {
	authMethods := make([]ssh.AuthMethod, 0, 1)
	switch srv.SFTP.Auth.Type {
	case "password":
		authMethods = append(authMethods, ssh.Password(srv.SFTP.Auth.Password))
	case "private_key":
		keyBytes, err := os.ReadFile(srv.SFTP.Auth.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("read private key for server %q: %w", srv.ID, err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil && srv.SFTP.Auth.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(srv.SFTP.Auth.Passphrase))
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key for server %q: %w", srv.ID, err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default:
		return nil, fmt.Errorf("unsupported sftp auth type %q for server %q", srv.SFTP.Auth.Type, srv.ID)
	}

	return &ssh.ClientConfig{
		User:            srv.SFTP.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

func ParseHTMLModlist(html string, warnf func(string, ...any)) []ParsedMod {
	rows := modRowPattern.FindAllStringSubmatch(html, -1)
	mods := make([]ParsedMod, 0, len(rows))
	for _, row := range rows {
		body := row[1]
		dn := ""
		if match := displayNamePattern.FindStringSubmatch(body); len(match) > 1 {
			dn = strings.TrimSpace(tagPattern.ReplaceAllString(match[1], ""))
		}
		link := ""
		if match := linkPattern.FindStringSubmatch(body); len(match) > 1 {
			link = strings.TrimSpace(match[1])
		}
		id := extractWorkshopID(link)
		if !digitsPattern.MatchString(id) {
			if warnf != nil {
				warnf("skipping mod row with invalid workshop id", "display_name", dn, "link", link, "workshop_id", id)
			}
			continue
		}
		mods = append(mods, ParsedMod{
			DisplayName: dn,
			Link:        link,
			WorkshopID:  id,
			FolderSlug:  SlugifyFolder(dn, id),
		})
	}
	return mods
}

func extractWorkshopID(link string) string {
	idx := strings.Index(link, "?")
	if idx < 0 || idx == len(link)-1 {
		return ""
	}
	query := link[idx+1:]
	for _, pair := range strings.Split(query, "&") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 && parts[0] == "id" {
			return parts[1]
		}
	}
	return ""
}

func HashModset(ids []string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(sum[:])
}

func SlugifyFolder(displayName, workshopID string) string {
	slug := strings.ToLower(strings.TrimSpace(displayName))
	slug = spacePattern.ReplaceAllString(slug, "-")
	slug = nonSlugPattern.ReplaceAllString(slug, "")
	slug = dashPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "mod-" + workshopID
	}
	return slug
}

func ApplyPollResult(st *state.State, serverID string, result PollResult) {
	server := st.Servers[serverID]
	if server.SyncedMods == nil {
		server.SyncedMods = map[string]time.Time{}
	}
	previousHash := server.LastModsetHash
	server.LastModIDs = append([]string(nil), result.SortedIDs...)
	server.LastModsetHash = result.ModsetHash
	if previousHash != "" && previousHash != result.ModsetHash {
		server.NeedsModUpdate = true
		server.Stage = state.StagePlanning
	}
	st.Servers[serverID] = server

	for _, mod := range result.Mods {
		existing := st.Mods[mod.WorkshopID]
		existing.DisplayName = mod.DisplayName
		existing.FolderSlug = mod.FolderSlug
		st.Mods[mod.WorkshopID] = existing
	}
}
