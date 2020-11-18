package builder

import (
	"context"

	"github.com/buildpacks/imgutil"

	pubcfg "github.com/buildpacks/pack/config"
)

type ImageFetcher interface {
	// Fetch fetches an image by resolving it both remotely and locally depending on provided parameters.
	// If daemon is true, it will look return a `local.Image`. Pull, applicable only when daemon is true, will
	// attempt to pull a remote image first.
	Fetch(ctx context.Context, name string, daemon bool, pullPolicy pubcfg.PullPolicy) (imgutil.Image, error)
}

type ImageFetcherWrapper struct {
	fetcher ImageFetcher
}

func NewImageFetcherWrapper(fetcher ImageFetcher) *ImageFetcherWrapper {
	return &ImageFetcherWrapper{
		fetcher: fetcher,
	}
}

func (w *ImageFetcherWrapper) Fetch(
	ctx context.Context,
	name string,
	daemon bool,
	pullPolicy pubcfg.PullPolicy,
) (Inspectable, error) {
	return w.fetcher.Fetch(ctx, name, daemon, pullPolicy)
}
