package entigo

// OptionKey identifies a named entry in the ServiceOptions context map.
type OptionKey string

const (
	OptionKeyServiceContext OptionKey = "service_context"
	OptionKeyAudit          OptionKey = "audit"
	OptionKeyCache          OptionKey = "cache"
	OptionKeyDB             OptionKey = "db"
	OptionKeyReplicaDB      OptionKey = "replica_db"
	OptionKeyRedis          OptionKey = "redis"
	OptionKeyTracer           OptionKey = "tracer"
	OptionKeyContextExtractor OptionKey = "context_extractor"
)

// ServiceOptions holds configuration for entity services, including debug
// flags and a key-value context map for injecting dependencies like DB
// connections, cache, tracer, and audit service.
type ServiceOptions struct {
	DebugMode    bool
	DebugSQLMode bool
	Context      map[OptionKey]any
}

// NewServiceOptions creates a new ServiceOptions with an empty context map.
func NewServiceOptions() *ServiceOptions {
	return &ServiceOptions{
		Context: make(map[OptionKey]any),
	}
}

// Clone creates a deep copy of the ServiceOptions, including the context map.
func (s *ServiceOptions) Clone() *ServiceOptions {
	newContext := make(map[OptionKey]any, len(s.Context))
	for key, value := range s.Context {
		newContext[key] = value
	}
	return &ServiceOptions{
		DebugMode:    s.DebugMode,
		DebugSQLMode: s.DebugSQLMode,
		Context:      newContext,
	}
}

// With sets a key-value pair in the context map and returns the receiver
// for method chaining.
func (s *ServiceOptions) With(key OptionKey, value any) *ServiceOptions {
	s.Context[key] = value
	return s
}

// Get retrieves the value for the given key from the context map.
func (s *ServiceOptions) Get(key OptionKey) any {
	return s.Context[key]
}
