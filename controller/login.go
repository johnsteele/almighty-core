package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"net/http"
	"net/url"

	"fmt"

	"github.com/almighty/almighty-core/account"

	"github.com/almighty/almighty-core/app"
	"github.com/almighty/almighty-core/errors"
	"github.com/almighty/almighty-core/jsonapi"
	"github.com/almighty/almighty-core/log"
	"github.com/almighty/almighty-core/login"
	"github.com/almighty/almighty-core/test"
	"github.com/almighty/almighty-core/token"
	"github.com/goadesign/goa"
	e "github.com/pkg/errors"
)

type loginConfiguration interface {
	GetKeycloakEndpointAuth(*goa.RequestData) (string, error)
	GetKeycloakEndpointToken(*goa.RequestData) (string, error)
	GetKeycloakClientID() string
	GetKeycloakSecret() string
	IsPostgresDeveloperModeEnabled() bool
	GetKeycloakTestUserName() string
	GetKeycloakTestUserSecret() string
}

// LoginController implements the login resource.
type LoginController struct {
	*goa.Controller
	auth          login.KeycloakOAuthService
	tokenManager  token.Manager
	configuration loginConfiguration
}

// NewLoginController creates a login controller.
func NewLoginController(service *goa.Service, auth *login.KeycloakOAuthProvider, tokenManager token.Manager, configuration loginConfiguration) *LoginController {
	return &LoginController{Controller: service.NewController("login"), auth: auth, tokenManager: tokenManager, configuration: configuration}
}

// Authorize runs the authorize action.
func (c *LoginController) Authorize(ctx *app.AuthorizeLoginContext) error {
	authEndpoint, err := c.configuration.GetKeycloakEndpointAuth(ctx.RequestData)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "Unable to get Keycloak auth endpoint URL")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError("unable to get Keycloak auth endpoint URL "+err.Error()))
	}

	tokenEndpoint, err := c.configuration.GetKeycloakEndpointToken(ctx.RequestData)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "Unable to get Keycloak token endpoint URL")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError("unable to get Keycloak token endpoint URL "+err.Error()))
	}
	return c.auth.Perform(ctx, authEndpoint, tokenEndpoint)
}

// Refresh obtain a new access token using the refresh token.
func (c *LoginController) Refresh(ctx *app.RefreshLoginContext) error {
	refreshToken := ctx.Payload.RefreshToken
	if refreshToken == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewBadParameterError("refresh_token", nil).Expected("not nil"))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	endpoint, err := c.configuration.GetKeycloakEndpointToken(ctx.RequestData)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "Unable to get Keycloak token endpoint URL")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError("unable to get Keycloak token endpoint URL "+err.Error()))
	}
	res, err := client.PostForm(endpoint, url.Values{
		"client_id":     {c.configuration.GetKeycloakClientID()},
		"client_secret": {c.configuration.GetKeycloakSecret()},
		"refresh_token": {*refreshToken},
		"grant_type":    {"refresh_token"},
	})
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError("Error when obtaining token "+err.Error()))
	}
	switch res.StatusCode {
	case 200:
		// OK
	case 401:
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError(res.Status+" "+readBody(res.Body)))
	case 400:
		return jsonapi.JSONErrorResponse(ctx, errors.NewBadParameterError(readBody(res.Body), nil))
	default:
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(res.Status+" "+readBody(res.Body)))
	}

	token, err := readToken(res, ctx)
	if err != nil {
		return err
	}

	return ctx.OK(&app.AuthToken{Token: token})
}

func readBody(body io.ReadCloser) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(body)
	return buf.String()
}

func readToken(res *http.Response, ctx jsonapi.InternalServerError) (*app.TokenData, error) {
	// Read the json out of the response body
	buf := new(bytes.Buffer)
	io.Copy(buf, res.Body)
	res.Body.Close()
	jsonString := strings.TrimSpace(buf.String())

	var token app.TokenData
	err := json.Unmarshal([]byte(jsonString), &token)
	if err != nil {
		return nil, jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(fmt.Sprintf("error when unmarshal json with access token %s ", jsonString)+err.Error()))
	}
	return &token, nil
}

// Generate obtain the access token from Keycloak for the test user
func (c *LoginController) Generate(ctx *app.GenerateLoginContext) error {
	if !c.configuration.IsPostgresDeveloperModeEnabled() {
		log.Error(ctx, map[string]interface{}{
			"method": "Generate",
		}, "Postgres developer mode not enabled")
		jerrors, _ := jsonapi.ErrorToJSONAPIErrors(goa.ErrUnauthorized("Postgres developer mode not enabled"))
		return ctx.Unauthorized(jerrors)
	}

	var scopes []account.Identity
	scopes = append(scopes, test.TestIdentity)
	scopes = append(scopes, test.TestObserverIdentity)

	client := &http.Client{Timeout: 10 * time.Second}

	username := c.configuration.GetKeycloakTestUserName()

	endpoint, err := c.configuration.GetKeycloakEndpointToken(ctx.RequestData)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "Unable to get Keycloak token endpoint URL")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError("unable to get Keycloak token endpoint URL "+err.Error()))
	}

	res, err := client.PostForm(endpoint, url.Values{
		"client_id":     {c.configuration.GetKeycloakClientID()},
		"client_secret": {c.configuration.GetKeycloakSecret()},
		"username":      {username},
		"password":      {c.configuration.GetKeycloakTestUserSecret()},
		"grant_type":    {"password"},
	})
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError("error when obtaining token "+err.Error()))
	}

	token, err := readToken(res, ctx)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"tokenEndpoint": res,
			"err":           err,
		}, "Error when unmarshal json with access token")
		return jsonapi.JSONErrorResponse(ctx, e.Wrap(err, "Error when unmarshal json with access token"))
	}

	var tokens app.AuthTokenCollection
	tokens = append(tokens, &app.AuthToken{Token: token})
	// Creates the testuser user and identity if they don't yet exist
	c.auth.CreateKeycloakUser(*token.AccessToken, ctx)

	return ctx.OK(tokens)
}
