package base

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ridge/must"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/logger"
	"go.uber.org/zap"
)

// FIXME (wojciech): return types.ErrImageDoesNotExist if image dos not exist in registry

// NewDockerInitializer creates new initializer getting base images from docker registry
func NewDockerInitializer() Initializer {
	return &dockerInitializer{}
}

type dockerInitializer struct {
}

// Initialize fetches image from docker registry and integrates it inside directory
func (f *dockerInitializer) Init(ctx context.Context, buildKey types.BuildKey, dstDir string) error {
	log := logger.Get(ctx)

	token, err := f.authorize(ctx, buildKey.Name)
	if err != nil {
		return err
	}
	layers, err := f.layers(ctx, token, buildKey)
	if err != nil {
		return err
	}
	for _, digest := range layers {
		log.Info("Incrementing filesystem", zap.String("digest", digest))
		if err := f.increment(ctx, token, buildKey.Name, digest, dstDir); err != nil {
			return err
		}
	}
	return nil
}

func (f *dockerInitializer) authorize(ctx context.Context, imageName string) (string, error) {
	resp, err := http.DefaultClient.Do(must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", imageName), nil)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	data := struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"` // nolint: tagliatelle
	}{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	if data.Token != "" {
		return data.Token, nil
	}
	return data.AccessToken, nil
}

func (f *dockerInitializer) layers(ctx context.Context, token string, buildKey types.BuildKey) ([]string, error) {
	req := must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://registry-1.docker.io/v2/library/%s/manifests/%s", buildKey.Name, buildKey.Tag), nil))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	data := struct {
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}{}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	layers := make([]string, 0, len(data.Layers))
	for _, l := range data.Layers {
		layers = append(layers, l.Digest)
	}
	return layers, nil
}

func (f *dockerInitializer) increment(ctx context.Context, token, imageName, digest string, dstDir string) error {
	// FIXME (wojciech): ensure that we don't remove linked or symlinked content when using RemoveAll

	req := must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://registry-1.docker.io/v2/library/%s/blobs/%s", imageName, digest), nil))
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	hasher := sha256.New()
	gr, err := gzip.NewReader(io.TeeReader(resp.Body, hasher))
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	del := map[string]bool{}
	added := map[string]bool{}
	// for hardlinks linked inode has to exist so we have to create them only when target is created
	links := map[string]string{}
loop:
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			break loop
		case err != nil:
			return err
		case header == nil:
			continue
		}
		dst := filepath.Join(dstDir, header.Name)

		if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
			return err
		}

		switch {
		case filepath.Base(header.Name) == ".wh..wh..plnk":
			// just ignore this
		case filepath.Base(header.Name) == ".wh..wh..opq":
			// It means that content in this directory created by earlier layers should not be visible
			// so content created earlier should be deleted
			dir := filepath.Dir(dst)
			files, err := os.ReadDir(dir)
			if err != nil {
				return err
			}
			for _, f := range files {
				toDelete := filepath.Join(dir, f.Name())
				if added[toDelete] {
					continue
				}
				if err := os.RemoveAll(toDelete); err != nil {
					return err
				}
			}
		case strings.HasPrefix(filepath.Base(dst), ".wh."):
			// delete or mark to delete corresponding file
			toDelete := filepath.Join(filepath.Dir(dst), strings.TrimPrefix(filepath.Base(dst), ".wh."))
			delete(added, toDelete)
			delete(links, toDelete)
			if err := os.RemoveAll(toDelete); err != nil {
				if os.IsNotExist(err) {
					del[toDelete] = true
					continue
				}
				return err
			}
		case del[dst]:
			delete(del, dst)
			delete(added, dst)
			delete(links, dst)
		case header.Typeflag == tar.TypeDir:
			added[dst] = true
			if err := os.MkdirAll(dst, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case header.Typeflag == tar.TypeReg:
			added[dst] = true
			if err := func() error {
				f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
				if err != nil {
					return err
				}
				defer f.Close()

				_, err = io.Copy(f, tr)
				if errors.Is(err, io.EOF) {
					err = nil
				}
				return err
			}(); err != nil {
				return err
			}
		case header.Typeflag == tar.TypeSymlink:
			added[dst] = true
			if err := os.Symlink(header.Linkname, dst); err != nil {
				return err
			}
		case header.Typeflag == tar.TypeLink:
			added[dst] = true
			links[dst] = filepath.Join(dstDir, header.Linkname)
		default:
			return fmt.Errorf("unsupported file type: %d", header.Typeflag)
		}

		// symlinks can't be chowned - in this case os.ErrNotExist is returned
		if err := os.Chown(dst, header.Uid, header.Gid); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	for dst, linkName := range links {
		if err := os.Link(linkName, dst); err != nil {
			return err
		}
	}

	computedDigest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if computedDigest != digest {
		return fmt.Errorf("digest doesn't match, expected: %s, got: %s", digest, computedDigest)
	}
	return nil
}
