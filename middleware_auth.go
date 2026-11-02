package mrc

import (
    "errors"
    "strings"
)

// TokenExtractor extracts a raw JWT from the current request.
type TokenExtractor func(
    c *Context,
) (string, error)

var ErrInvalidAuthorizationHeader = errors.New(
    "mrc: invalid authorization header",
)

// ExtractBearerToken extracts JWT from an Authorization header.
func ExtractBearerToken(
    authorization string,
) (string, error) {
    parts := strings.Fields(authorization)

    if len(parts) != 2 {
        return "", ErrInvalidAuthorizationHeader
    }

    if !strings.EqualFold(parts[0], "Bearer") {
        return "", ErrInvalidAuthorizationHeader
    }

    return parts[1], nil
}

// Middleware creates JWT authentication middleware.
func (a *AuthManager) Middleware(
    extract TokenExtractor,
) (HandlerFunc, error) {
    if a == nil {
        return nil, ErrAuthNotConfigured
    }

    if extract == nil {
        return nil, errors.New(
            "mrc: token extractor is required",
        )
    }

    return func(c *Context) {
        if c == nil || !c.active {
            return
        }

        tokenString, err := extract(c)
        if err != nil || strings.TrimSpace(tokenString) == "" {
            c.Abort()

            _ = c.JSON(StatusUnauthorized, H{
                "error": "unauthorized",
            })

            return
        }

        claims, err := a.ParseToken(tokenString)
        if err != nil {
            // Do not expose JWT parsing details to clients.
            c.Abort()

            _ = c.JSON(StatusUnauthorized, H{
                "error": "unauthorized",
            })

            return
        }

        c.SetClaims(claims)
    }, nil
}
