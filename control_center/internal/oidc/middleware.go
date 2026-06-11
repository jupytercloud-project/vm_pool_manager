package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const ClaimsKey contextKey = "oidcClaims"

// JWKS cache
var (
	jwksMu      sync.RWMutex
	jwksCache   map[string]interface{}
	jwksFetched time.Time
)

func issuerURL() string {
	u := os.Getenv("OIDC_ISSUER")
	if u == "" {
		u = "http://localhost:5556/dex"
	}
	return u
}

func fetchJWKS() (map[string]interface{}, error) {
	jwksMu.RLock()
	if time.Since(jwksFetched) < 5*time.Minute && jwksCache != nil {
		defer jwksMu.RUnlock()
		return jwksCache, nil
	}
	jwksMu.RUnlock()

	resp, err := http.Get(issuerURL() + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()

	var discovery struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, err
	}

	resp2, err := http.Get(discovery.JWKSURI)
	if err != nil {
		return nil, fmt.Errorf("jwks fetch: %w", err)
	}
	defer resp2.Body.Close()

	var jwks map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	jwksMu.Lock()
	jwksCache = jwks
	jwksFetched = time.Now()
	jwksMu.Unlock()

	return jwks, nil
}

// keyFunc fetches the public key for JWT validation using the JWKS endpoint.
// For simplicity, we use the RS256 public key from the JWKS.
func keyFunc(token *jwt.Token) (interface{}, error) {
	jwks, err := fetchJWKS()
	if err != nil {
		return nil, err
	}

	keys, ok := jwks["keys"].([]interface{})
	if !ok || len(keys) == 0 {
		return nil, fmt.Errorf("no keys in JWKS")
	}

	// Find matching key by kid
	kid, _ := token.Header["kid"].(string)
	for _, k := range keys {
		key, ok := k.(map[string]interface{})
		if !ok {
			continue
		}
		if kid != "" && key["kid"] != kid {
			continue
		}
		pub, err := jwkToPublicKey(key)
		if err != nil {
			continue
		}
		return pub, nil
	}
	return nil, fmt.Errorf("no matching key found")
}

// UnaryInterceptor validates the Bearer JWT from gRPC metadata.
// Routes in skipMethods bypass auth (public endpoints like AttribVMService).
func UnaryInterceptor(skipMethods []string) grpc.UnaryServerInterceptor {
	skipSet := make(map[string]bool, len(skipMethods))
	for _, m := range skipMethods {
		skipSet[m] = true
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if skipSet[info.FullMethod] {
			return handler(ctx, req)
		}
		ctx, err := validateAndInject(ctx)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor validates the Bearer JWT for streaming RPCs.
func StreamInterceptor(skipMethods []string) grpc.StreamServerInterceptor {
	skipSet := make(map[string]bool, len(skipMethods))
	for _, m := range skipMethods {
		skipSet[m] = true
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if skipSet[info.FullMethod] {
			return handler(srv, ss)
		}
		ctx, err := validateAndInject(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ss, ctx})
	}
}

func validateAndInject(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auths := md.Get("authorization")
	if len(auths) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	tokenStr := auths[0]
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		tokenStr = tokenStr[7:]
	}

	claims, err := ParseToken(tokenStr)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	return context.WithValue(ctx, ClaimsKey, claims), nil
}

// ParseToken valide un JWT OIDC (signature via JWKS, issuer, expiration, algorithme RS256)
// et renvoie ses claims. Exporté pour être réutilisé par le middleware HTTP REST.
func ParseToken(tokenStr string) (jwt.MapClaims, error) {
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		tokenStr = tokenStr[7:]
	}
	claims := jwt.MapClaims{}
	// WithValidMethods (RS256) ferme l'alg-confusion (un token alg=HS256 abuserait la clé
	// RSA publique comme secret HMAC). NB : l'issuer n'est PAS imposé ici car OIDC_ISSUER
	// (défaut localhost) ne correspond pas toujours à l'issuer réel annoncé par Dex
	// (IP machine) ; l'activer casserait la connexion. À durcir quand OIDC_ISSUER sera aligné.
	_, err := jwt.ParseWithClaims(tokenStr, claims, keyFunc,
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// ClaimsToContext injecte des claims validés dans un contexte (pour le pont HTTP→gRPC).
func ClaimsToContext(ctx context.Context, claims jwt.MapClaims) context.Context {
	return context.WithValue(ctx, ClaimsKey, claims)
}

// EmailFromContext extracts the email from validated JWT claims.
func EmailFromContext(ctx context.Context) (string, bool) {
	claims, ok := ctx.Value(ClaimsKey).(jwt.MapClaims)
	if !ok {
		return "", false
	}
	email, ok := claims["email"].(string)
	return email, ok
}

// GroupsFromContext extracts groups claim (used to determine admin role).
func GroupsFromContext(ctx context.Context) []string {
	claims, ok := ctx.Value(ClaimsKey).(jwt.MapClaims)
	if !ok {
		return nil
	}
	raw, ok := claims["groups"].([]interface{})
	if !ok {
		return nil
	}
	groups := make([]string, 0, len(raw))
	for _, g := range raw {
		if s, ok := g.(string); ok {
			groups = append(groups, s)
		}
	}
	return groups
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
