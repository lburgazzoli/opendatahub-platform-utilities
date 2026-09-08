package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
)

var (
	errURLScheme        = errors.New("URL scheme must be HTTPS")
	errURLHost          = errors.New("URL host is required")
	errURLCredentials   = errors.New("URL credentials are not supported")
	errHTTPStatus       = errors.New("unexpected HTTP status")
	errManifestTooLarge = errors.New("manifest exceeds size limit")
	errAbsolutePath     = errors.New("path must be absolute")
	errInvalidPath      = errors.New("path is invalid")
)

// Source loads raw bytes from a location.
type Source interface {
	Load(ctx context.Context) ([]byte, error)
}

// URLSource loads content from an HTTPS URL.
type URLSource struct {
	URL    *url.URL
	Client *http.Client
}

// NewURLSource parses raw as an HTTPS URL and returns a URLSource using
// http.DefaultClient.
func NewURLSource(raw string) (URLSource, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return URLSource{}, fmt.Errorf("parse URL: %w", err)
	}

	switch {
	case u.Scheme != "https":
		return URLSource{}, fmt.Errorf("%w: got %q", errURLScheme, u.Scheme)
	case u.Host == "":
		return URLSource{}, fmt.Errorf("%w: %q", errURLHost, raw)
	case u.User != nil:
		return URLSource{}, errURLCredentials
	default:
		return URLSource{URL: u, Client: http.DefaultClient}, nil
	}
}

const manifestReadLimit = 50 << 20

// Load fetches the content from the URL.
func (s URLSource) Load(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", s.URL.Redacted(), err)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", s.URL.Redacted(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %w: HTTP %d", s.URL.Redacted(), errHTTPStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, manifestReadLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read body %s: %w", s.URL.Redacted(), err)
	}
	if int64(len(body)) > manifestReadLimit {
		return nil, fmt.Errorf("fetch %s: %w", s.URL.Redacted(), errManifestTooLarge)
	}

	return body, nil
}

// FileSource loads content from a filesystem path.
// Uses fs.FS for testability; pass os.DirFS("/") for real filesystem access.
type FileSource struct {
	FS   fs.FS
	Path string
}

// NewFileSource creates a FileSource that reads from the real filesystem.
// path must be absolute.
func NewFileSource(path string) (FileSource, error) {
	if len(path) == 0 || path[0] != '/' {
		return FileSource{}, fmt.Errorf("%w: %q", errAbsolutePath, path)
	}

	stripped := path[1:]
	if !fs.ValidPath(stripped) {
		return FileSource{}, fmt.Errorf("%w: %q", errInvalidPath, path)
	}

	return FileSource{FS: os.DirFS("/"), Path: stripped}, nil
}

// Load reads the file content.
func (s FileSource) Load(_ context.Context) ([]byte, error) {
	if !fs.ValidPath(s.Path) {
		return nil, fmt.Errorf("%w: %q", errInvalidPath, s.Path)
	}

	data, err := fs.ReadFile(s.FS, s.Path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", s.Path, err)
	}

	return data, nil
}
