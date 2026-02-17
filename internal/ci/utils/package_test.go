package utils

import (
	"testing"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/stretchr/testify/require"
)

func TestGetPackageName(t *testing.T) {
	require.Equal(t, GetPackageName("java", &config.LanguageConfig{
		Cfg: map[string]any{
			"groupID":    "com.example",
			"artifactID": "test",
		},
	}), "com.example.test")
	require.Equal(t, GetPackageName("terraform", &config.LanguageConfig{
		Cfg: map[string]any{
			"author":      "ryan",
			"packageName": "test",
		},
	}), "ryan/test")
	require.Equal(t, GetPackageName("go", &config.LanguageConfig{
		Cfg: map[string]any{
			"author":      "ryan",
			"packageName": "test",
		},
	}), "test")
}

func TestGetRegistryName(t *testing.T) {
	require.Equal(t, GetRegistryName("go"), "go")
	require.Equal(t, GetRegistryName("python"), "pypi")
	require.Equal(t, GetRegistryName("typescript"), "npm")
	require.Equal(t, GetRegistryName("php"), "packagist")
	require.Equal(t, GetRegistryName("ruby"), "gems")
	require.Equal(t, GetRegistryName("java"), "sonatype")
	require.Equal(t, GetRegistryName("terraform"), "terraform")
	require.Equal(t, GetRegistryName("go"), "go")
}
