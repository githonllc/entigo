package entigo

import "context"

// ContextKey is a typed key for storing and retrieving values from context.Context.
// Using a distinct type prevents collisions with keys from other packages.
// Consumers can define their own domain-specific keys using this type.
type ContextKey string

// ActorInfo holds the identity and metadata of the actor performing an operation.
// It is extracted from context.Context by a ContextExtractor and used for
// access control and audit logging.
type ActorInfo struct {
	ActorType  ActorType
	ActorID    ID
	IdentityID ID
	IsAdmin    bool
	IPAddress  string
	UserAgent  string
}

// ContextExtractor extracts actor information from a request context.
// Implement this interface to customize how entigo resolves the current user,
// admin status, IP address, and other metadata from your application's context.
//
// Inject via ServiceOptions:
//
//	opts.With(entigo.OptionKeyContextExtractor, myExtractor)
type ContextExtractor interface {
	Extract(ctx context.Context) ActorInfo
}

// defaultContextExtractor reads actor info from context using the built-in
// ContextKey constants. This is the fallback when no custom extractor is provided.
type defaultContextExtractor struct{}

func (d defaultContextExtractor) Extract(ctx context.Context) ActorInfo {
	info := ActorInfo{}
	if v, ok := ctx.Value(CtxKeyUserID).(ID); ok {
		info.ActorType = ActorTypeUser
		info.ActorID = v
	} else if v, ok := ctx.Value(CtxKeyApiKeyID).(ID); ok {
		info.ActorType = ActorTypeApiKey
		info.ActorID = v
	}
	info.IdentityID, _ = ctx.Value(CtxKeyIdentityID).(ID)
	info.IsAdmin, _ = ctx.Value(CtxKeyIsAdmin).(bool)
	info.IPAddress, _ = ctx.Value(CtxKeyRealIP).(string)
	info.UserAgent, _ = ctx.Value(CtxKeyUserAgent).(string)
	return info
}

// Built-in context key constants. These are used by the defaultContextExtractor
// and the ginx.RequireContext helper. If you provide a custom ContextExtractor,
// you do not need to use these keys.
const (
	CtxKeyUserID     ContextKey = "entigo.user_id"
	CtxKeyIdentityID ContextKey = "entigo.identity_id"
	CtxKeyApiKeyID   ContextKey = "entigo.api_key_id"
	CtxKeyIsAdmin    ContextKey = "entigo.is_admin"
	CtxKeyRealIP     ContextKey = "entigo.real_ip"
	CtxKeyUserAgent  ContextKey = "entigo.user_agent"
	CtxKeyClientIP   ContextKey = "entigo.client_ip"
)
