package pr

import (
	"errors"
	"testing"
)

func TestGetResolver(t *testing.T) {
	tests := []struct {
		name         string
		platform     Platform
		wantResolver bool
		wantErr      error
	}{
		{
			name:         "GitHub returns resolver",
			platform:     PlatformGitHub,
			wantResolver: true,
			wantErr:      nil,
		},
		{
			name:         "GitLab returns unsupported error",
			platform:     PlatformGitLab,
			wantResolver: false,
			wantErr:      ErrUnsupportedPlatform,
		},
		{
			name:         "BitBucket returns unsupported error",
			platform:     PlatformBitBucket,
			wantResolver: false,
			wantErr:      ErrUnsupportedPlatform,
		},
		{
			name:         "unknown platform returns error",
			platform:     Platform("unknown"),
			wantResolver: false,
			wantErr:      nil, // different error, not ErrUnsupportedPlatform
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &PRInfo{Platform: tt.platform}
			resolver, err := GetResolver(info)

			if tt.wantResolver {
				if resolver == nil {
					t.Error("GetResolver() returned nil resolver, want non-nil")
				}
				if err != nil {
					t.Errorf("GetResolver() error = %v, want nil", err)
				}
				if resolver != nil && resolver.Platform() != tt.platform {
					t.Errorf("resolver.Platform() = %v, want %v", resolver.Platform(), tt.platform)
				}
			} else {
				if resolver != nil {
					t.Errorf("GetResolver() returned resolver %v, want nil", resolver)
				}
				if err == nil {
					t.Error("GetResolver() error = nil, want error")
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("GetResolver() error = %v, want error containing %v", err, tt.wantErr)
				}
			}
		})
	}
}
