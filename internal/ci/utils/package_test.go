package utils

import (
	"testing"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/stretchr/testify/require"
)

func TestGetPackageName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "com.example.test", GetPackageName("java", &config.LanguageConfig{
		Cfg: map[string]any{
			"groupID":    "com.example",
			"artifactID": "test",
		},
	}))
	require.Equal(t, "ryan/test", GetPackageName("terraform", &config.LanguageConfig{
		Cfg: map[string]any{
			"author":      "ryan",
			"packageName": "test",
		},
	}))
	require.Equal(t, "test", GetPackageName("go", &config.LanguageConfig{
		Cfg: map[string]any{
			"author":      "ryan",
			"packageName": "test",
		},
	}))
}

func TestGetRegistryName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "go", GetRegistryName("go"))
	require.Equal(t, "pypi", GetRegistryName("python"))
	require.Equal(t, "npm", GetRegistryName("typescript"))
	require.Equal(t, "packagist", GetRegistryName("php"))
	require.Equal(t, "gems", GetRegistryName("ruby"))
	require.Equal(t, "sonatype", GetRegistryName("java"))
	require.Equal(t, "terraform", GetRegistryName("terraform"))
	require.Equal(t, "go", GetRegistryName("go"))
}
