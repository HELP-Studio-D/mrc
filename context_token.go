package mrc

import "time"

type Token struct {
	Token     string
	ExpiresAt time.Time
	IssuedAt  time.Time

	Claims map[string]string
}

// SetToken stores the authenticated token in the request context.
func (c *Context) SetToken(token *Token) {
	c.Set(contextTokenKey, token)
}

// Token retrieves the authenticated token.
func (c *Context) Token() (*Token, bool) {
	value, ok := c.Get(contextTokenKey)
	if !ok {
		return nil, false
	}

	token, ok := value.(*Token)
	return token, ok
}

// MustToken retrieves the token or panics.
func (c *Context) MustToken() any {
	return c.MustGet(contextTokenKey)
}

// ClearToken removes the authenticated token.
func (c *Context) ClearToken() {
	c.Delete(contextTokenKey)
}
