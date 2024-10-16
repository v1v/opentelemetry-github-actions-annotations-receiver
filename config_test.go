package opentelemetrygithubactionsannotationsreceiver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	opentelemetrygithubactionsannotationsreceiver "github.com/v1v/opentelemetry-github-actions-annotations-receiver"
)

func TestConfigValidateSuccess(t *testing.T) {
	config := &opentelemetrygithubactionsannotationsreceiver.Config{
		Path: "/test",
		GitHubAuth: opentelemetrygithubactionsannotationsreceiver.GitHubAuth{
			Token: "token",
		},
	}
	err := config.Validate()
	assert.NoError(t, err)
}

func TestConfigValidateMissingGitHubTokenShouldFail(t *testing.T) {
	config := &opentelemetrygithubactionsannotationsreceiver.Config{
		Path: "/test",
	}
	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, "either github_auth.token or github_auth.app_id must be set", err.Error())
}

func TestConfigValidateMalformedPathShouldFail(t *testing.T) {
	// arrange
	config := &opentelemetrygithubactionsannotationsreceiver.Config{
		Path: "lol !",
		GitHubAuth: opentelemetrygithubactionsannotationsreceiver.GitHubAuth{
			Token: "fake-token",
		},
	}

	// act
	err := config.Validate()

	// assert
	assert.EqualError(t, err, "path must be a valid URL: parse \"lol !\": invalid URI for request")
}

func TestConfigValidateAbsolutePathShouldFail(t *testing.T) {
	config := &opentelemetrygithubactionsannotationsreceiver.Config{
		Path: "https://www.example.com/events",
		GitHubAuth: opentelemetrygithubactionsannotationsreceiver.GitHubAuth{
			Token: "fake-token",
		},
	}
	err := config.Validate()
	assert.EqualError(t, err, "path must be a relative URL. e.g. \"/events\"")
}

func TestConfigValidateNoAuthShouldFail(t *testing.T) {
	config := &opentelemetrygithubactionsannotationsreceiver.Config{}
	err := config.Validate()
	assert.EqualError(t, err, "either github_auth.token or github_auth.app_id must be set")
}

func TestConfigValidateGitHubAppShouldSucceed(t *testing.T) {
	config := &opentelemetrygithubactionsannotationsreceiver.Config{
		GitHubAuth: opentelemetrygithubactionsannotationsreceiver.GitHubAuth{
			AppID:          123,
			InstallationID: 456,
			PrivateKey:     "fake",
		},
	}
	err := config.Validate()

	assert.NoError(t, err)
}

func TestConfigValidateGitHubAppPrivateKeyPathShouldSucceed(t *testing.T) {
	config := &opentelemetrygithubactionsannotationsreceiver.Config{
		GitHubAuth: opentelemetrygithubactionsannotationsreceiver.GitHubAuth{
			AppID:          123,
			InstallationID: 456,
			PrivateKeyPath: "fake",
		},
	}
	err := config.Validate()

	assert.NoError(t, err)
}

func TestConfigValidateOnlyAppIdShouldFail(t *testing.T) {
	// arrange
	config := &opentelemetrygithubactionsannotationsreceiver.Config{
		GitHubAuth: opentelemetrygithubactionsannotationsreceiver.GitHubAuth{
			AppID: 123,
		},
	}

	// act
	err := config.Validate()

	// assert
	assert.EqualError(t, err, "github_auth.installation_id must be set if github_auth.app_id is set; either github_auth.private_key or github_auth.private_key_path must be set if github_auth.app_id is set")
}
