package mrc

import "errors"

var ErrClaimsNotFound = errors.New(
	"mrc: authentication claims not found",
)

// Claims represents authenticated user information.
type Claims struct {
	Subject     string         `json:"sub"`
	Roles       []string       `json:"roles,omitempty"`
	Permissions []string       `json:"permissions,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

// NewClaims creates claims for a user.
func NewClaims(subject string) *Claims {
	return &Claims{
		Subject: subject,
		Data:    make(map[string]any),
	}
}

// HasRole checks whether the user has a role.
func (c *Claims) HasRole(role string) bool {
	if c == nil {
		return false
	}

	for _, current := range c.Roles {
		if current == role {
			return true
		}
	}

	return false
}

// HasAnyRole checks whether the user has at least one role.
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}

	return false
}

// HasAllRoles checks whether the user has every specified role.
func (c *Claims) HasAllRoles(roles ...string) bool {
	for _, role := range roles {
		if !c.HasRole(role) {
			return false
		}
	}

	return true
}

// AddRole adds a role if it does not already exist.
func (c *Claims) AddRole(role string) {
	if c == nil || role == "" || c.HasRole(role) {
		return
	}

	c.Roles = append(c.Roles, role)
}

// RemoveRole removes a role.
func (c *Claims) RemoveRole(role string) {
	if c == nil {
		return
	}

	for i, current := range c.Roles {
		if current == role {
			c.Roles = append(c.Roles[:i], c.Roles[i+1:]...)
			return
		}
	}
}

// HasPermission checks whether the user has permission.
func (c *Claims) HasPermission(permission string) bool {
	if c == nil {
		return false
	}

	for _, current := range c.Permissions {
		if current == permission {
			return true
		}
	}

	return false
}

// HasAnyPermission checks whether the user has at least
// one of the specified permissions.
func (c *Claims) HasAnyPermission(permissions ...string) bool {
	for _, permission := range permissions {
		if c.HasPermission(permission) {
			return true
		}
	}

	return false
}

// HasAllPermissions checks whether the user has all permissions.
func (c *Claims) HasAllPermissions(permissions ...string) bool {
	for _, permission := range permissions {
		if !c.HasPermission(permission) {
			return false
		}
	}

	return true
}

// AddPermission adds a permission if it does not exist.
func (c *Claims) AddPermission(permission string) {
	if c == nil || permission == "" || c.HasPermission(permission) {
		return
	}

	c.Permissions = append(c.Permissions, permission)
}

// RemovePermission removes a permission.
func (c *Claims) RemovePermission(permission string) {
	if c == nil {
		return
	}

	for i, current := range c.Permissions {
		if current == permission {
			c.Permissions = append(
				c.Permissions[:i],
				c.Permissions[i+1:]...,
			)
			return
		}
	}
}

// Set adds custom claim data.
func (c *Claims) Set(key string, value any) {
	if c == nil || key == "" {
		return
	}

	if c.Data == nil {
		c.Data = make(map[string]any)
	}

	c.Data[key] = value
}

// Value retrieves custom claim data.
func (c *Claims) Value(key string) (any, bool) {
	if c == nil || c.Data == nil {
		return nil, false
	}

	value, exists := c.Data[key]
	return value, exists
}

// SetClaims stores authenticated claims in Context.
func (c *Context) SetClaims(claims *Claims) {
	if claims == nil {
		c.Delete(contextClaimsKey)
		return
	}

	c.Set(contextClaimsKey, claims)
}

// Claims retrieves authenticated claims.
func (c *Context) Claims() (*Claims, bool) {
	value, exists := c.Get(contextClaimsKey)
	if !exists {
		return nil, false
	}

	claims, ok := value.(*Claims)
	return claims, ok && claims != nil
}

// MustClaims retrieves claims or panics.
func (c *Context) MustClaims() *Claims {
	claims, exists := c.Claims()
	if !exists {
		panic(ErrClaimsNotFound)
	}

	return claims
}

// ClearClaims removes authenticated claims.
func (c *Context) ClearClaims() {
	c.Delete(contextClaimsKey)
}
