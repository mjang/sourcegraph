// Package jscontext contains functionality for information we pass down into
// the JS webapp.
package jscontext

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hooks"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/assetsutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/siteid"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/webhooks"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/schema"
)

// BillingPublishableKey is the publishable (non-secret) API key for the billing system, if any.
var BillingPublishableKey string

type authProviderInfo struct {
	IsBuiltin         bool   `json:"isBuiltin"`
	DisplayName       string `json:"displayName"`
	ServiceType       string `json:"serviceType"`
	AuthenticationURL string `json:"authenticationURL"`
	ServiceID         string `json:"serviceID"`
}

// GenericPasswordPolicy a generic password policy that holds password requirements
type authPasswordPolicy struct {
	Enabled                   bool `json:"enabled"`
	NumberOfSpecialCharacters int  `json:"numberOfSpecialCharacters"`
	RequireAtLeastOneNumber   bool `json:"requireAtLeastOneNumber"`
	RequireUpperAndLowerCase  bool `json:"requireUpperAndLowerCase"`
}
type UserLatestSettings struct {
	ID       int32                      // the unique ID of this settings value
	Contents graphqlbackend.JSONCString // the raw JSON (with comments and trailing commas allowed)
}
type UserOrganization struct {
	ID          graphql.ID
	Name        string
	DisplayName *string
	URL         string
	SettingsURL *string
}
type UserEmail struct {
	Email     string
	IsPrimary bool
	Verified  bool
}

type CurrentUser struct {
	ID                  graphql.ID
	DatabaseID          int32
	Username            string
	AvatarURL           string
	DisplayName         string
	SiteAdmin           bool
	URL                 string
	SettingsURL         string
	ViewerCanAdminister bool
	Tags                []string
	TosAccepted         bool
	Searchable          bool

	Organizations  []*UserOrganization
	CanSignOut     *bool
	Emails         []UserEmail
	LatestSettings *UserLatestSettings
}

// JSContext is made available to JavaScript code via the
// "sourcegraph/app/context" module.
//
// 🚨 SECURITY: This struct is sent to all users regardless of whether or
// not they are logged in, for example on an auth.public=false private
// server. Including secret fields here is OK if it is based on the user's
// authentication above, but do not include e.g. hard-coded secrets about
// the server instance here as they would be sent to anonymous users.
type JSContext struct {
	AppRoot        string            `json:"appRoot,omitempty"`
	ExternalURL    string            `json:"externalURL,omitempty"`
	XHRHeaders     map[string]string `json:"xhrHeaders"`
	UserAgentIsBot bool              `json:"userAgentIsBot"`
	AssetsRoot     string            `json:"assetsRoot"`
	Version        string            `json:"version"`

	IsAuthenticatedUser bool         `json:"isAuthenticatedUser"`
	CurrentUser         *CurrentUser `json:"CurrentUser"`

	SentryDSN     *string               `json:"sentryDSN"`
	OpenTelemetry *schema.OpenTelemetry `json:"openTelemetry"`

	SiteID        string `json:"siteID"`
	SiteGQLID     string `json:"siteGQLID"`
	Debug         bool   `json:"debug"`
	NeedsSiteInit bool   `json:"needsSiteInit"`
	EmailEnabled  bool   `json:"emailEnabled"`

	Site              schema.SiteConfiguration `json:"site"` // public subset of site configuration
	LikelyDockerOnMac bool                     `json:"likelyDockerOnMac"`
	NeedServerRestart bool                     `json:"needServerRestart"`
	DeployType        string                   `json:"deployType"`

	SourcegraphDotComMode bool `json:"sourcegraphDotComMode"`

	BillingPublishableKey string `json:"billingPublishableKey,omitempty"`

	AccessTokensAllow conf.AccessTokenAllow `json:"accessTokensAllow"`

	AllowSignup bool `json:"allowSignup"`

	ResetPasswordEnabled bool `json:"resetPasswordEnabled"`

	ExternalServicesUserMode string `json:"externalServicesUserMode"`

	AuthMinPasswordLength int                `json:"authMinPasswordLength"`
	AuthPasswordPolicy    authPasswordPolicy `json:"authPasswordPolicy"`

	AuthProviders []authProviderInfo `json:"authProviders"`

	Branding *schema.Branding `json:"branding"`

	BatchChangesEnabled                bool `json:"batchChangesEnabled"`
	BatchChangesDisableWebhooksWarning bool `json:"batchChangesDisableWebhooksWarning"`
	BatchChangesWebhookLogsEnabled     bool `json:"batchChangesWebhookLogsEnabled"`

	ExecutorsEnabled                         bool `json:"executorsEnabled"`
	CodeIntelAutoIndexingEnabled             bool `json:"codeIntelAutoIndexingEnabled"`
	CodeIntelAutoIndexingAllowGlobalPolicies bool `json:"codeIntelAutoIndexingAllowGlobalPolicies"`

	CodeInsightsEnabled bool `json:"codeInsightsEnabled"`

	RedirectUnsupportedBrowser bool `json:"RedirectUnsupportedBrowser"`

	ProductResearchPageEnabled bool `json:"productResearchPageEnabled"`

	ExperimentalFeatures schema.ExperimentalFeatures `json:"experimentalFeatures"`

	EnableLegacyExtensions bool `json:"enableLegacyExtensions"`

	LicenseInfo *hooks.LicenseInfo `json:"licenseInfo"`

	OutboundRequestLogLimit int `json:"outboundRequestLogLimit"`

	DisableFeedbackSurvey bool `json:"disableFeedbackSurvey"`
}

// NewJSContextFromRequest populates a JSContext struct from the HTTP
// request.
func NewJSContextFromRequest(req *http.Request, db database.DB) JSContext {
	ctx := req.Context()
	a := sgactor.FromContext(ctx)

	headers := make(map[string]string)
	headers["x-sourcegraph-client"] = globals.ExternalURL().String()
	headers["X-Requested-With"] = "Sourcegraph" // required for httpapi to use cookie auth

	// Propagate Cache-Control no-cache and max-age=0 directives
	// to the requests made by our client-side JavaScript. This is
	// not a perfect parser, but it catches the important cases.
	if cc := req.Header.Get("cache-control"); strings.Contains(cc, "no-cache") || strings.Contains(cc, "max-age=0") {
		headers["Cache-Control"] = "no-cache"
	}

	siteID := siteid.Get()

	// Show the site init screen?
	globalState, err := db.GlobalState().Get(ctx)
	needsSiteInit := err == nil && !globalState.Initialized

	// Auth providers
	var authProviders []authProviderInfo
	for _, p := range providers.Providers() {
		if p.Config().Github != nil && p.Config().Github.Hidden {
			continue
		}
		info := p.CachedInfo()
		if info != nil {
			authProviders = append(authProviders, authProviderInfo{
				IsBuiltin:         p.Config().Builtin != nil,
				DisplayName:       info.DisplayName,
				ServiceType:       p.ConfigID().Type,
				AuthenticationURL: info.AuthenticationURL,
				ServiceID:         info.ServiceID,
			})
		}
	}

	pp := conf.AuthPasswordPolicy()

	var authPasswordPolicy authPasswordPolicy
	authPasswordPolicy.Enabled = pp.Enabled
	authPasswordPolicy.NumberOfSpecialCharacters = pp.NumberOfSpecialCharacters
	authPasswordPolicy.RequireAtLeastOneNumber = pp.RequireAtLeastOneNumber
	authPasswordPolicy.RequireUpperAndLowerCase = pp.RequireUpperandLowerCase

	var sentryDSN *string
	siteConfig := conf.Get().SiteConfiguration

	if siteConfig.Log != nil && siteConfig.Log.Sentry != nil && siteConfig.Log.Sentry.Dsn != "" {
		sentryDSN = &siteConfig.Log.Sentry.Dsn
	}

	var openTelemetry *schema.OpenTelemetry
	if clientObservability := siteConfig.ObservabilityClient; clientObservability != nil {
		openTelemetry = clientObservability.OpenTelemetry
	}

	var licenseInfo *hooks.LicenseInfo
	var user *types.User
	if !a.IsAuthenticated() {
		licenseInfo = hooks.GetLicenseInfo(false)
	} else {
		// Ignore err as we don't care if user does not exist
		user, _ = a.User(ctx, db.Users())
		licenseInfo = hooks.GetLicenseInfo(user != nil && user.SiteAdmin)
	}

	// 🚨 SECURITY: This struct is sent to all users regardless of whether or
	// not they are logged in, for example on an auth.public=false private
	// server. Including secret fields here is OK if it is based on the user's
	// authentication above, but do not include e.g. hard-coded secrets about
	// the server instance here as they would be sent to anonymous users.
	return JSContext{
		ExternalURL:         globals.ExternalURL().String(),
		XHRHeaders:          headers,
		UserAgentIsBot:      isBot(req.UserAgent()),
		AssetsRoot:          assetsutil.URL("").String(),
		Version:             version.Version(),
		IsAuthenticatedUser: a.IsAuthenticated(),
		CurrentUser:         createCurrentUser(ctx, user, db),

		SentryDSN:                  sentryDSN,
		OpenTelemetry:              openTelemetry,
		RedirectUnsupportedBrowser: siteConfig.RedirectUnsupportedBrowser,
		Debug:                      env.InsecureDev,
		SiteID:                     siteID,

		SiteGQLID: string(graphqlbackend.SiteGQLID()),

		NeedsSiteInit:     needsSiteInit,
		EmailEnabled:      conf.CanSendEmail(),
		Site:              publicSiteConfiguration(),
		LikelyDockerOnMac: likelyDockerOnMac(),
		NeedServerRestart: globals.ConfigurationServerFrontendOnly.NeedServerRestart(),
		DeployType:        deploy.Type(),

		SourcegraphDotComMode: envvar.SourcegraphDotComMode(),

		BillingPublishableKey: BillingPublishableKey,

		// Experiments. We pass these through explicitly, so we can
		// do the default behavior only in Go land.
		AccessTokensAllow: conf.AccessTokensAllow(),

		ResetPasswordEnabled: userpasswd.ResetPasswordEnabled(),

		ExternalServicesUserMode: conf.ExternalServiceUserMode().String(),

		AllowSignup: conf.AuthAllowSignup(),

		AuthMinPasswordLength: conf.AuthMinPasswordLength(),
		AuthPasswordPolicy:    authPasswordPolicy,

		AuthProviders: authProviders,

		Branding: globals.Branding(),

		BatchChangesEnabled:                enterprise.BatchChangesEnabledForUser(ctx, db) == nil,
		BatchChangesDisableWebhooksWarning: conf.Get().BatchChangesDisableWebhooksWarning,
		BatchChangesWebhookLogsEnabled:     webhooks.LoggingEnabled(conf.Get()),

		ExecutorsEnabled:                         conf.ExecutorsEnabled(),
		CodeIntelAutoIndexingEnabled:             conf.CodeIntelAutoIndexingEnabled(),
		CodeIntelAutoIndexingAllowGlobalPolicies: conf.CodeIntelAutoIndexingAllowGlobalPolicies(),

		CodeInsightsEnabled: enterprise.IsCodeInsightsEnabled(),

		ProductResearchPageEnabled: conf.ProductResearchPageEnabled(),

		ExperimentalFeatures: conf.ExperimentalFeatures(),

		EnableLegacyExtensions: conf.ExperimentalFeatures().EnableLegacyExtensions,

		LicenseInfo: licenseInfo,

		OutboundRequestLogLimit: conf.Get().OutboundRequestLogLimit,

		DisableFeedbackSurvey: conf.Get().DisableFeedbackSurvey,
	}
}

// createCurrentUser creates CurrentUser object which contains of types.User
// properties along with some extra data such as user emails, organisations,
// session information, etc.
func createCurrentUser(ctx context.Context, user *types.User, db database.DB) *CurrentUser {
	if user == nil {
		return nil
	}

	userResolver := graphqlbackend.NewUserResolver(db, user)

	siteAdmin, _ := userResolver.SiteAdmin(ctx)
	canAdminister, _ := userResolver.ViewerCanAdminister(ctx)
	tags, _ := userResolver.Tags(ctx)

	var canSignOut *bool
	if session, err := userResolver.Session(ctx); err == nil {
		*canSignOut = session.CanSignOut()
	}

	return &CurrentUser{
		ID:                  userResolver.ID(),
		DatabaseID:          userResolver.DatabaseID(),
		Username:            userResolver.Username(),
		AvatarURL:           derefString(userResolver.AvatarURL()),
		DisplayName:         derefString(userResolver.DisplayName()),
		SiteAdmin:           siteAdmin,
		URL:                 userResolver.URL(),
		SettingsURL:         derefString(userResolver.SettingsURL()),
		ViewerCanAdminister: canAdminister,
		Tags:                tags,
		TosAccepted:         userResolver.TosAccepted(ctx),
		Searchable:          userResolver.Searchable(ctx),
		Organizations:       resolveUserOrganizations(ctx, userResolver),
		CanSignOut:          canSignOut,
		Emails:              resolveUserEmails(ctx, userResolver),
		LatestSettings:      resolveLatestSettings(ctx, userResolver),
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func resolveUserOrganizations(ctx context.Context, user *graphqlbackend.UserResolver) []*UserOrganization {
	orgs, err := user.Organizations(ctx)
	if err != nil {
		return nil
	}
	userOrganizations := make([]*UserOrganization, 0, len(orgs.Nodes()))
	for _, org := range orgs.Nodes() {
		userOrganizations = append(userOrganizations, &UserOrganization{
			ID:          org.ID(),
			Name:        org.Name(),
			DisplayName: org.DisplayName(),
			URL:         org.URL(),
			SettingsURL: org.SettingsURL(),
		})
	}
	return userOrganizations
}

func resolveUserEmails(ctx context.Context, user *graphqlbackend.UserResolver) []UserEmail {
	emails, err := user.Emails(ctx)
	if err != nil {
		return nil
	}

	userEmails := make([]UserEmail, 0, len(emails))

	for _, emailResolver := range emails {
		userEmail := UserEmail{
			Email:     emailResolver.Email(),
			IsPrimary: emailResolver.IsPrimary(),
			Verified:  emailResolver.Verified(),
		}
		userEmails = append(userEmails, userEmail)
	}

	return userEmails
}

func resolveLatestSettings(ctx context.Context, user *graphqlbackend.UserResolver) *UserLatestSettings {
	settings, err := user.LatestSettings(ctx)
	if err != nil {
		return nil
	}
	return &UserLatestSettings{
		ID:       settings.ID(),
		Contents: settings.Contents(),
	}
}

// publicSiteConfiguration is the subset of the site.schema.json site
// configuration that is necessary for the web app and is not sensitive/secret.
func publicSiteConfiguration() schema.SiteConfiguration {
	c := conf.Get()
	updateChannel := c.UpdateChannel
	if updateChannel == "" {
		updateChannel = "release"
	}
	return schema.SiteConfiguration{
		AuthPublic:                  c.AuthPublic,
		UpdateChannel:               updateChannel,
		AuthzEnforceForSiteAdmins:   c.AuthzEnforceForSiteAdmins,
		DisableNonCriticalTelemetry: c.DisableNonCriticalTelemetry,
	}
}

var isBotPat = lazyregexp.New(`(?i:googlecloudmonitoring|pingdom.com|go .* package http|sourcegraph e2etest|bot|crawl|slurp|spider|feed|rss|camo asset proxy|http-client|sourcegraph-client)`)

func isBot(userAgent string) bool {
	return isBotPat.MatchString(userAgent)
}

func likelyDockerOnMac() bool {
	r := net.DefaultResolver
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	addrs, err := r.LookupHost(ctx, "host.docker.internal")
	if err != nil || len(addrs) == 0 {
		return false //  Assume we're not docker for mac.
	}
	return true
}
