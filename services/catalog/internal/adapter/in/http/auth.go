package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

var errNoToken = errors.New("нет токена")

type Authenticator interface {
	Authenticate(r *http.Request) (security.Principal, error)
}

type LocalTokens struct{}

func (LocalTokens) Authenticate(r *http.Request) (security.Principal, error) {
	raw, ok := bearer(r)
	if !ok {
		return security.Principal{}, errNoToken
	}
	role, id, found := strings.Cut(raw, ".")
	if !found {
		return security.Principal{}, errors.New("локальный токен имеет вид role.uuid")
	}
	sub, err := uuid.Parse(id)
	if err != nil {
		return security.Principal{}, fmt.Errorf("идентификатор в токене: %w", err)
	}
	return security.Principal{Sub: sub, Roles: []security.Role{security.Role(role)}}, nil
}

type JWTAuthenticator struct {
	jwks     keyfunc.Keyfunc
	issuer   string
	audience string
}

func NewJWTAuthenticator(ctx context.Context, jwksURL, issuer, audience string) (*JWTAuthenticator, error) {
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	return &JWTAuthenticator{jwks: jwks, issuer: issuer, audience: audience}, nil
}

func (a *JWTAuthenticator) Authenticate(r *http.Request) (security.Principal, error) {
	raw, ok := bearer(r)
	if !ok {
		return security.Principal{}, errNoToken
	}
	claims := jwt.MapClaims{}
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{"RS256", "ES256"}), jwt.WithExpirationRequired(), jwt.WithIssuer(a.issuer)}
	if a.audience != "" {
		opts = append(opts, jwt.WithAudience(a.audience))
	}
	if _, err := jwt.ParseWithClaims(raw, claims, a.jwks.Keyfunc, opts...); err != nil {
		return security.Principal{}, err
	}
	subject, _ := claims.GetSubject()
	sub, err := uuid.Parse(subject)
	if err != nil {
		return security.Principal{}, fmt.Errorf("sub в токене: %w", err)
	}
	return security.Principal{Sub: sub, Roles: realmRoles(claims)}, nil
}

func realmRoles(claims jwt.MapClaims) []security.Role {
	access, ok := claims["realm_access"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := access["roles"].([]any)
	if !ok {
		return nil
	}
	roles := make([]security.Role, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			roles = append(roles, security.Role(s))
		}
	}
	return roles
}

func bearer(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if len(header) < 8 || !strings.EqualFold(header[:7], "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(header[7:]), true
}

func Authenticate(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := auth.Authenticate(r)
			switch {
			case errors.Is(err, errNoToken):
				next.ServeHTTP(w, r)
			case err != nil:
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				writeProblem(w, r, http.StatusUnauthorized, "TOKEN_INVALID", "Токен не принят: "+err.Error())
			default:
				next.ServeHTTP(w, r.WithContext(security.WithPrincipal(r.Context(), principal)))
			}
		})
	}
}

func RequireRoles(roles ...security.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := security.PrincipalFrom(r.Context())
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeProblem(w, r, http.StatusUnauthorized, "TOKEN_MISSING", "Требуется аутентификация")
				return
			}
			for _, role := range roles {
				if principal.HasRole(role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeProblem(w, r, http.StatusForbidden, "ACCESS_DENIED", "Доступ запрещён")
		})
	}
}
