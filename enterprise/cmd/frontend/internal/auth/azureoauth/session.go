package azureoauth

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"golang.org/x/oauth2"
)

type sessionIssuerHelper struct {
	*extsvc.CodeHost
	db          database.DB
	clientID    string
	allowSignup bool
	// TODO: allowgroups
}

// TODO: Implement
func (s *sessionIssuerHelper) GetOrCreateUser(ctx context.Context, token *oauth2.Token, anonymousUserID, firstSourceURL, lastSourceURL string) (actr *actor.Actor, safeErrMsg string, err error) {
	// user, err :=
	l := log.Scoped("sessionIssuerHelper.GetOrCreateUser", "get or create user logger")
	l.Warn("here")

	err = errors.New("GetOrCreateUser: not implemented")
	return
}

func (s *sessionIssuerHelper) DeleteStateCookie(w http.ResponseWriter) {}

func (s *sessionIssuerHelper) SessionData(token *oauth2.Token) oauth.SessionData {
	return oauth.SessionData{}
}

func (s *sessionIssuerHelper) AuthSucceededEventName() database.SecurityEventName {
	return database.SecurityEventAzureDevOpsAuthSucceeded
}

func (s *sessionIssuerHelper) AuthFailedEventName() database.SecurityEventName {
	return database.SecurityEventAzureDevOpsAuthFailed
}

func (s *sessionIssuerHelper) newOauth2Client() (*azuredevops.Client, error) {
	httpCli, err := httpcli.ExternalClientFactory.Doer()
	if err != nil {
		return nil, errors.Wrap(err, "azuredevops: failed to create Oauth2 client")
	}

	// s.BaseURL

	// FIXME: Empty token
	auth := auth.OAuthBearerToken{}
	return azuredevops.NewClient("azuredevopsoauth", s.CodeHost.BaseURL, &auth, httpCli)
}

type key int

const userKey key = iota

func userFromContext(ctx context.Context) (*azuredevops.Profile, error) {
	user, ok := ctx.Value(userKey).(*azuredevops.Profile)
	if !ok {
		return nil, errors.Errorf("azuredevops: Context missing Azure DevOps user")
	}
	return user, nil
}
