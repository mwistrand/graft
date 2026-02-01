package pr

import (
	"context"
	"fmt"
)

// GetResolver returns the appropriate resolver for a PRInfo based on its platform.
func GetResolver(info *PRInfo) (Resolver, error) {
	switch info.Platform {
	case PlatformGitHub:
		return NewGitHubResolver(info.Host), nil
	case PlatformGitLab:
		return nil, fmt.Errorf("%w: GitLab support coming soon", ErrUnsupportedPlatform)
	case PlatformBitBucket:
		return nil, fmt.Errorf("%w: BitBucket support coming soon", ErrUnsupportedPlatform)
	default:
		return nil, fmt.Errorf("unknown platform: %s", info.Platform)
	}
}

// Resolve is a convenience function that parses and resolves a PR URL.
func Resolve(ctx context.Context, rawURL string) (*PRMetadata, error) {
	info, err := Parse(rawURL)
	if err != nil {
		return nil, err
	}

	resolver, err := GetResolver(info)
	if err != nil {
		return nil, err
	}

	return resolver.Resolve(ctx, info)
}
