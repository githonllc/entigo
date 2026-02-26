package entigo

import "context"

// ActorType identifies the type of actor performing an audited operation.
type ActorType string

const (
	ActorTypeSystem ActorType = "SYSTEM"
	ActorTypeUser   ActorType = "USER"
	ActorTypeApiKey ActorType = "API_KEY"
)

// AuditService records audit log events for entity operations.
type AuditService interface {
	LogEvent(ctx context.Context, entry *AuditLogEvent)
}

// AuditLogEventOption is a functional option for configuring an AuditLogEvent.
type AuditLogEventOption func(*AuditLogEvent)

// AuditLogEvent represents a single auditable action with contextual metadata.
type AuditLogEvent struct {
	ActorType    ActorType
	ActorID      ID
	IdentityID   ID
	ResourceType string
	ResourceID   ID
	Action       string
	Status       string
	Details      map[string]any
	IPAddress    string
	UserAgent    string
	ErrorMessage string
}

// ToMap converts the event to a flat map, suitable for structured logging or storage.
func (e AuditLogEvent) ToMap() map[string]any {
	return map[string]any{
		"actor_type":    string(e.ActorType),
		"actor_id":      e.ActorID,
		"identity_id":   e.IdentityID,
		"resource_type": e.ResourceType,
		"resource_id":   e.ResourceID,
		"action":        e.Action,
		"status":        e.Status,
		"details":       e.Details,
		"ip_address":    e.IPAddress,
		"user_agent":    e.UserAgent,
		"error_message": e.ErrorMessage,
	}
}

// WithActorType sets the actor type on the audit event.
func WithActorType(actorType ActorType) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.ActorType = actorType
	}
}

// WithActorID sets the actor ID on the audit event.
func WithActorID(actorID ID) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.ActorID = actorID
	}
}

// WithIdentityID sets the identity ID on the audit event.
func WithIdentityID(identityID ID) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.IdentityID = identityID
	}
}

// WithResourceType sets the resource type (e.g., "user", "device") on the audit event.
func WithResourceType(resourceType string) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.ResourceType = resourceType
	}
}

// WithResourceID sets the resource ID on the audit event.
func WithResourceID(resourceID ID) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.ResourceID = resourceID
	}
}

// WithAction sets the action name (e.g., "create", "delete") on the audit event.
func WithAction(action string) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.Action = action
	}
}

// WithStatus sets the status (e.g., "success", "failure") on the audit event.
func WithStatus(status string) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.Status = status
	}
}

// WithDetails sets the additional details map on the audit event.
func WithDetails(details map[string]any) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.Details = details
	}
}

// WithIPAddress sets the client IP address on the audit event.
func WithIPAddress(ipAddress string) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.IPAddress = ipAddress
	}
}

// WithUserAgent sets the client user agent string on the audit event.
func WithUserAgent(userAgent string) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.UserAgent = userAgent
	}
}

// WithErrorMessage sets the error message on the audit event, typically used
// when the audited operation failed.
func WithErrorMessage(errorMessage string) AuditLogEventOption {
	return func(e *AuditLogEvent) {
		e.ErrorMessage = errorMessage
	}
}

// NewAuditLogEvent creates a new AuditLogEvent with the given functional options applied.
func NewAuditLogEvent(opts ...AuditLogEventOption) *AuditLogEvent {
	e := &AuditLogEvent{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// dummyAuditService is a no-operation audit service that discards all events.
type dummyAuditService struct{}

func (s *dummyAuditService) LogEvent(ctx context.Context, entry *AuditLogEvent) {}

// NewDummyAuditService creates an AuditService that silently discards all audit events.
// Useful for testing or when audit logging is not needed.
func NewDummyAuditService() AuditService {
	return &dummyAuditService{}
}
