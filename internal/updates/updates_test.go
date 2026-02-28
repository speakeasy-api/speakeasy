package updates

import (
	"testing"

	"github.com/google/go-github/v63/github"
	"github.com/stretchr/testify/assert"
)

func TestIsStableRelease(t *testing.T) {
	tests := []struct {
		name       string
		release    *github.RepositoryRelease
		wantStable bool
	}{
		{
			name: "stable release",
			release: &github.RepositoryRelease{
				TagName:    github.String("v1.2.3"),
				Draft:      github.Bool(false),
				Prerelease: github.Bool(false),
			},
			wantStable: true,
		},
		{
			name: "pre-release tag with dash suffix",
			release: &github.RepositoryRelease{
				TagName:    github.String("v1.2.3-pre.0"),
				Draft:      github.Bool(false),
				Prerelease: github.Bool(false),
			},
			wantStable: false,
		},
		{
			name: "pre-release tag with alpha suffix",
			release: &github.RepositoryRelease{
				TagName:    github.String("v1.2.3-alpha.1"),
				Draft:      github.Bool(false),
				Prerelease: github.Bool(false),
			},
			wantStable: false,
		},
		{
			name: "pre-release tag with rc suffix",
			release: &github.RepositoryRelease{
				TagName:    github.String("v2.0.0-rc.1"),
				Draft:      github.Bool(false),
				Prerelease: github.Bool(false),
			},
			wantStable: false,
		},
		{
			name: "github marked as pre-release",
			release: &github.RepositoryRelease{
				TagName:    github.String("v1.2.3"),
				Draft:      github.Bool(false),
				Prerelease: github.Bool(true),
			},
			wantStable: false,
		},
		{
			name: "draft release",
			release: &github.RepositoryRelease{
				TagName:    github.String("v1.2.3"),
				Draft:      github.Bool(true),
				Prerelease: github.Bool(false),
			},
			wantStable: false,
		},
		{
			name: "invalid tag",
			release: &github.RepositoryRelease{
				TagName:    github.String("not-a-version"),
				Draft:      github.Bool(false),
				Prerelease: github.Bool(false),
			},
			wantStable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStableRelease(tt.release)
			assert.Equal(t, tt.wantStable, got)
		})
	}
}
