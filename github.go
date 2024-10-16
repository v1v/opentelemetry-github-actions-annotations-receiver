package opentelemetrygithubactionsannotationsreceiver

import (
	"encoding/base64"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v66/github"
)

func createGitHubClient(githubAuth GitHubAuth) (*github.Client, error) {
	if githubAuth.AppID != 0 {
		if githubAuth.PrivateKey != "" {
			var privateKey []byte
			privateKey, err := base64.StdEncoding.DecodeString(string(githubAuth.PrivateKey))
			if err != nil {
				privateKey = []byte(githubAuth.PrivateKey)
			}
			itr, err := ghinstallation.New(
				http.DefaultTransport,
				githubAuth.AppID,
				githubAuth.InstallationID,
				privateKey,
			)
			if err != nil {
				return &github.Client{}, err
			}
			return github.NewClient(&http.Client{Transport: itr}), nil
		} else {
			itr, err := ghinstallation.NewKeyFromFile(
				http.DefaultTransport,
				githubAuth.AppID,
				githubAuth.InstallationID,
				githubAuth.PrivateKeyPath,
			)
			if err != nil {
				return &github.Client{}, err
			}
			return github.NewClient(&http.Client{Transport: itr}), nil
		}
	} else {
		return github.NewClient(nil).WithAuthToken(string(githubAuth.Token)), nil
	}
}
