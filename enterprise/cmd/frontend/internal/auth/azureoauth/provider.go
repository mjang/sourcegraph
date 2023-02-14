package azureoauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dghubble/gologin"
	oauth2Login "github.com/dghubble/gologin/oauth2"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/schema"
)

func Init(logger log.Logger, db database.DB) {
	const pkgName = "azuredoauth"
	logger = logger.Scoped(pkgName, "Azure DevOps OAuth config watch")
	conf.ContributeValidator(func(cfg conftypes.SiteConfigQuerier) conf.Problems {
		_, problems := parseConfig(logger, cfg, db)
		return problems
	})

	go func() {
		conf.Watch(func() {
			newProviders, _ := parseConfig(logger, conf.Get(), db)
			if len(newProviders) == 0 {
				providers.Update(pkgName, nil)
				return
			}

			if err := licensing.Check(licensing.FeatureSSO); err != nil {
				logger.Error("Check license for SSO (Azure DevOps OAuth)", log.Error(err))
				providers.Update(pkgName, nil)
				return
			}

			newProvidersList := make([]providers.Provider, 0, len(newProviders))
			for _, p := range newProviders {
				newProvidersList = append(newProvidersList, p.Provider)
			}
			providers.Update(pkgName, newProvidersList)
		})
	}()
}

type Provider struct {
	*schema.AzureDevOpsAuthProvider
	providers.Provider
}

func parseConfig(logger log.Logger, cfg conftypes.SiteConfigQuerier, db database.DB) (ps []Provider, problems conf.Problems) {
	for _, pr := range cfg.SiteConfig().AuthProviders {
		if pr.AzureDevOps == nil {
			continue
		}

		provider, providerProblems := parseProvider(logger, pr.AzureDevOps, db, pr)
		problems = append(problems, conf.NewSiteProblems(providerProblems...)...)

		if provider == nil {
			continue
		}

		ps = append(ps, Provider{
			AzureDevOpsAuthProvider: pr.AzureDevOps,
			Provider:                provider,
		})
	}

	return ps, problems
}

func parseProvider(logger log.Logger, p *schema.AzureDevOpsAuthProvider, db database.DB, sourceCfg schema.AuthProviders) (provider *oauth.Provider, messages []string) {
	// TODO: Handle empty p.Url. Or do I need to? I have a default?
	// app url is vscode
	parsedURL, err := url.Parse(p.Url)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Failed to parse Azure DevOps URL %q. Login via this Azure instance will not work.", p.Url))
		return nil, messages
	}

	codeHost := extsvc.NewCodeHost(parsedURL, extsvc.KindAzureDevOps)

	// TODO: App secret vs client secret
	return oauth.NewProvider(oauth.ProviderOp{
		AuthPrefix: authPrefix,
		OAuth2Config: func() oauth2.Config {
			return oauth2.Config{
				ClientID:     p.ClientID,
				ClientSecret: p.ClientSecret,
				Scopes:       strings.Split(p.ApiScope, ","),
				Endpoint: oauth2.Endpoint{
					AuthURL: parsedURL.ResolveReference(&url.URL{
						Path: "/oauth2/authorize",
					}).String(),
					TokenURL: parsedURL.ResolveReference(&url.URL{
						Path: "/oauth2/token",
					}).String(),
				},
			}
		},
		SourceConfig: sourceCfg,
		// TODO: Use this util in other places where we have the function getStateConfig.
		StateConfig: oauth.GetStateConfig("azure-state-cookie"),
		ServiceID:   parsedURL.String(),
		ServiceType: extsvc.TypeAzureDevOps,
		Login:       loginHandler,
		Callback: func(config oauth2.Config) http.Handler {
			return callbackHandler(
				logger,
				&config,
				oauth.SessionIssuer(logger, db, &sessionIssuerHelper{
					db:          db,
					CodeHost:    codeHost,
					clientID:    p.ClientID,
					allowSignup: p.AllowSignup,
				}, sessionKey),
			)
		},
	}), messages
}

func loginHandler(c oauth2.Config) http.Handler {
	return oauth2Login.LoginHandler(&c, nil)
}

func callbackHandler(logger log.Logger, config *oauth2.Config, success http.Handler) http.Handler {
	success = azureDevOpsHandler(logger, config, success, gologin.DefaultFailureHandler)

	return oauth2Login.CallbackHandler(config, success, gologin.DefaultFailureHandler)
}

func azureDevOpsHandler(logger log.Logger, config *oauth2.Config, success, failure http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		token, err := oauth2Login.TokenFromContext(ctx)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}

		// TODO: Finish implementation
		_, err = azureDevOpsClientFromAuthURL(config.Endpoint.AuthURL, token.AccessToken)
		if err != nil {
			ctx = gologin.WithError(ctx, errors.Errorf("could not parse AuthURL %s", config.Endpoint.AuthURL))
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}

		// TODO: PRobably don't need this
		// user, err := azureClient.GetUser(ctx, "")

		// FIXME: Implement this.
		// err = validateResponse(user, err)
		// if err != nil {
		// 	// TODO: Copy pasta
		// 	// TODO: Prefer a more general purpose fix, potentially
		// 	// https://github.com/sourcegraph/sourcegraph/pull/20000
		// 	logger.Warn("invalid response", log.Error(err))
		// }
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		// ctx = withUser(ctx, user)
		success.ServeHTTP(w, req.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// TODO: Implement this.
func azureDevOpsClientFromAuthURL(authURL, oauthToken string) (*azuredevops.Client, error) {
	baseURL, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}
	baseURL.Path = ""
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	// TODO: What urn do we need? Or do we even need it?
	return azuredevops.NewClientProvider(urnAzureDevOpsOAuth, baseURL, nil).GetOAuthClient(oauthToken), nil
}

const authPrefix = auth.AuthURLPrefix + "/azuredevops"
const sessionKey = "azuredevopsoauth@0"
const urnAzureDevOpsOAuth = "AzureDevOpsOAuth"
