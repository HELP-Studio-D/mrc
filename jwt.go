package mrc

import (
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

// jwtClaims is the internal JWT representation.
type jwtClaims struct {
    Roles       []string       `json:"roles,omitempty"`
    Permissions []string       `json:"permissions,omitempty"`
    Data        map[string]any `json:"data,omitempty"`

    jwt.RegisteredClaims
}

// GenerateToken generates a signed JWT.
func (a *AuthManager) GenerateToken(
    claims *Claims,
) (string, error) {
    if a == nil {
        return "", ErrAuthNotConfigured
    }

    if claims == nil || strings.TrimSpace(claims.Subject) == "" {
        return "", ErrInvalidClaims
    }

    now := time.Now().UTC()

    payload := jwtClaims{
        Roles:       append([]string(nil), claims.Roles...),
        Permissions: append([]string(nil), claims.Permissions...),
        Data:        cloneClaimsData(claims.Data),

        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    a.issuer,
            Subject:   claims.Subject,
            Audience:  append(jwt.ClaimStrings(nil), a.audience...),
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(a.accessTTL)),
        },
    }

    token := jwt.NewWithClaims(
        jwt.SigningMethodHS256,
        payload,
    )

    signed, err := token.SignedString(a.signingKey)
    if err != nil {
        return "", fmt.Errorf("mrc: signing JWT: %w", err)
    }

    return signed, nil
}

// ParseToken validates a JWT and returns its claims.
func (a *AuthManager) ParseToken(
    tokenString string,
) (*Claims, error) {
    if a == nil {
        return nil, ErrAuthNotConfigured
    }

    if strings.TrimSpace(tokenString) == "" {
        return nil, ErrTokenMissing
    }

    payload := new(jwtClaims)

    options := []jwt.ParserOption{
        jwt.WithValidMethods([]string{
            jwt.SigningMethodHS256.Alg(),
        }),
        jwt.WithExpirationRequired(),
        jwt.WithIssuedAt(),
        jwt.WithLeeway(a.leeway),
    }

    if a.issuer != "" {
        options = append(
            options,
            jwt.WithIssuer(a.issuer),
        )
    }

    if len(a.audience) > 0 {
        options = append(
            options,
            jwt.WithAudience(a.audience...),
        )
    }

    token, err := jwt.ParseWithClaims(
        tokenString,
        payload,
        func(token *jwt.Token) (any, error) {
            // Never accept an unexpected signing algorithm.
            if token.Method != jwt.SigningMethodHS256 {
                return nil, ErrInvalidToken
            }

            return a.signingKey, nil
        },
        options...,
    )

    if err != nil {
        if errors.Is(err, jwt.ErrTokenExpired) {
            return nil, fmt.Errorf(
                "%w: %v",
                ErrTokenExpired,
                err,
            )
        }

        return nil, fmt.Errorf(
            "%w: %v",
            ErrInvalidToken,
            err,
        )
    }

    if token == nil || !token.Valid {
        return nil, ErrInvalidToken
    }

    if strings.TrimSpace(payload.Subject) == "" {
        return nil, ErrInvalidClaims
    }

    return &Claims{
        Subject:     payload.Subject,
        Roles:       payload.Roles,
        Permissions: payload.Permissions,
        Data:        payload.Data,
    }, nil
}

// cloneClaimsData creates an independent map for JWT serialization.
func cloneClaimsData(
    source map[string]any,
) map[string]any {
    if source == nil {
        return nil
    }

    result := make(map[string]any, len(source))

    for key, value := range source {
        result[key] = value
    }

    return result
}
