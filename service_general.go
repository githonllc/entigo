package entigo

import "log/slog"

// GeneralService defines the common interface shared by all services,
// providing access to the logger, tracer, cache, audit, context extractor,
// and service options.
type GeneralService interface {
	GetServiceName() string
	GetServiceOptions() *ServiceOptions
	GetTracer() Tracer
	GetCacheService() CacheService
	GetAuditService() AuditService
	GetContextExtractor() ContextExtractor
	GetLogger() *slog.Logger
}

// GeneralServiceImpl provides a default implementation of GeneralService.
// Embed this in concrete service types to inherit logging, tracing, caching,
// and audit capabilities.
type GeneralServiceImpl struct {
	Logger    *slog.Logger
	options   *ServiceOptions
	tracer    Tracer
	cache     CacheService
	audit     AuditService
	ctxExtr   ContextExtractor
}

// NewGeneralService creates a GeneralServiceImpl from the given options.
// Dependencies (tracer, cache, audit) are extracted from the options context
// map with safe fallbacks: NoopTracer, DummyCache, and DummyAuditService.
// An optional logger can be passed; otherwise slog.Default() is used.
func NewGeneralService(options *ServiceOptions, logger ...*slog.Logger) *GeneralServiceImpl {
	// Determine logger
	var l *slog.Logger
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	} else {
		l = slog.Default()
	}

	// Extract tracer with safe fallback
	var tracer Tracer
	if t, ok := options.Get(OptionKeyTracer).(Tracer); ok && t != nil {
		tracer = t
	} else {
		tracer = NoopTracer{}
	}

	// Extract cache with safe fallback
	var cache CacheService
	if c, ok := options.Get(OptionKeyCache).(CacheService); ok && c != nil {
		cache = c
	} else {
		cache = NewDummyCache()
	}

	// Extract audit with safe fallback
	var audit AuditService
	if a, ok := options.Get(OptionKeyAudit).(AuditService); ok && a != nil {
		audit = a
	} else {
		audit = NewDummyAuditService()
	}

	// Extract context extractor with safe fallback
	var ctxExtr ContextExtractor
	if e, ok := options.Get(OptionKeyContextExtractor).(ContextExtractor); ok && e != nil {
		ctxExtr = e
	} else {
		ctxExtr = defaultContextExtractor{}
	}

	service := &GeneralServiceImpl{
		Logger:  l,
		options: options,
		tracer:  tracer,
		cache:   cache,
		audit:   audit,
		ctxExtr: ctxExtr,
	}

	return service
}

// GetServiceName returns the default service name.
func (s *GeneralServiceImpl) GetServiceName() string {
	return "EntityService"
}

// GetServiceOptions returns the service options used to configure this service.
func (s *GeneralServiceImpl) GetServiceOptions() *ServiceOptions {
	return s.options
}

// GetTracer returns the tracer used for distributed tracing.
func (s *GeneralServiceImpl) GetTracer() Tracer {
	return s.tracer
}

// GetCacheService returns the cache service.
func (s *GeneralServiceImpl) GetCacheService() CacheService {
	return s.cache
}

// GetAuditService returns the audit logging service.
func (s *GeneralServiceImpl) GetAuditService() AuditService {
	return s.audit
}

// GetContextExtractor returns the context extractor used to resolve actor info.
func (s *GeneralServiceImpl) GetContextExtractor() ContextExtractor {
	return s.ctxExtr
}

// GetLogger returns the structured logger.
func (s *GeneralServiceImpl) GetLogger() *slog.Logger {
	return s.Logger
}
