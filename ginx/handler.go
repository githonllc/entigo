package ginx

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/githonllc/entigo"
	"gorm.io/gorm"
)

// HandlerHooks provides lifecycle hooks for BaseHandler CRUD operations.
// Each hook receives the Gin context and the entity being processed.
type HandlerHooks[T entigo.Entity] struct {
	BeforeCreate func(c *gin.Context, entity T) error
	AfterCreate  func(c *gin.Context, entity T) error
	BeforeUpdate func(c *gin.Context, entity T) error
	AfterUpdate  func(c *gin.Context, entity T) error
	BeforePatch  func(c *gin.Context, input any) error
	AfterPatch   func(c *gin.Context, input any) error
}

// BaseHandler provides generic CRUD HTTP handler methods for any entity type.
// It uses the entigo.EntityService for data access and the entigo.Converter
// for automatic DTO transformation.
type BaseHandler[S entigo.EntityService[T], T entigo.Entity] struct {
	Logger    *slog.Logger
	Hooks     HandlerHooks[T]
	service   S
	converter *entigo.Converter[T]
	tracer    entigo.Tracer
}

// NewBaseHandler creates a new BaseHandler for the given entity service.
// An optional logger can be provided; otherwise slog.Default() is used.
// The tracer is obtained from the service.
func NewBaseHandler[S entigo.EntityService[T], T entigo.Entity](service S, logger ...*slog.Logger) BaseHandler[S, T] {
	var l *slog.Logger
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	} else {
		l = slog.Default()
	}

	return BaseHandler[S, T]{
		Logger:    l,
		service:   service,
		converter: entigo.NewConverter[T](service.GetZeroValue()),
		tracer:    service.GetTracer(),
	}
}

// GetResponseData converts entity data to its response DTO representation.
// It handles both single entities and slices of entities.
func (h *BaseHandler[S, T]) GetResponseData(data any) (any, error) {
	if data == nil {
		return nil, nil
	}

	// single entity output
	entity, ok := data.(T)
	if ok {
		output, err := h.converter.ToResponse(entity)
		if err != nil {
			h.Logger.Error("failed to convert entity to response", "error", err, "entity", entity)
			return nil, NewInternalServerError(err.Error())
		}
		return output, nil
	}

	// list of entities output
	entityList, ok := data.([]T)
	if ok && entityList != nil {
		output, err := h.converter.ToListResponse(entityList)
		if err != nil {
			h.Logger.Error("failed to convert entity to response", "error", err, "entityList", entityList)
			return nil, NewInternalServerError(err.Error())
		}
		return output, nil
	}

	return data, nil
}

// GetResponseDataAsSlice converts entity data to a slice of response DTOs.
// Single entities are wrapped in a one-element slice.
func (h *BaseHandler[S, T]) GetResponseDataAsSlice(data any) ([]any, error) {
	if data == nil {
		return nil, nil
	}

	// single entity output
	entity, ok := data.(T)
	if ok {
		output, err := h.converter.ToResponse(entity)
		if err != nil {
			h.Logger.Error("failed to convert entity to response", "error", err, "entity", entity)
			return nil, NewInternalServerError(err.Error())
		}
		return []any{output}, nil
	}

	// list of entities output
	entityList, ok := data.([]T)
	if ok && entityList != nil {
		output, err := h.converter.ToListResponse(entityList)
		if err != nil {
			h.Logger.Error("failed to convert entity to response", "error", err, "entityList", entityList)
			return nil, NewInternalServerError(err.Error())
		}
		return output, nil
	}

	return []any{}, nil
}

// Success converts the data to its response DTO and sends a success response.
func (h *BaseHandler[S, T]) Success(c *gin.Context, message string, data any, args ...any) {
	output, err := h.GetResponseData(data)
	if err != nil {
		h.Error(c, ErrInternalServer)
		return
	}
	SendOK(c, message, output, args...)
}

// Error logs the error and sends an appropriate HTTP error response.
func (h *BaseHandler[S, T]) Error(c *gin.Context, err error) {
	h.Logger.Warn(err.Error())
	WriteError(c, err)
}

// GetID retrieves an entity ID by checking the URL parameter, query parameter,
// and context value in that order.
func (h *BaseHandler[S, T]) GetID(c *gin.Context, idParamName string) (entigo.ID, error) {
	idStr := c.Param(idParamName)
	if idStr == "" {
		idStr = c.Query(idParamName)
		if idStr == "" {
			idStr = c.GetString(idParamName)
			if idStr == "" {
				return 0, fmt.Errorf("ID not found (empty string)")
			}
		}
	}

	id, err := entigo.ParseID(idStr)
	if err != nil {
		h.Logger.Error("invalid ID", "error", err, "id", idStr)
		return 0, fmt.Errorf("invalid ID %s", idStr)
	}

	return id, nil
}

// GetIDParam retrieves an entity ID from a URL path parameter.
func (h *BaseHandler[S, T]) GetIDParam(c *gin.Context, idParamName string) (entigo.ID, error) {
	idStr := c.Param(idParamName)
	if idStr == "" {
		return entigo.ID(0), fmt.Errorf("invalid ID (empty string)")
	}

	id, err := entigo.ParseID(idStr)
	if err != nil {
		h.Logger.Error("invalid ID", "error", err, "id", idStr)
		return 0, fmt.Errorf("invalid ID %s", idStr)
	}

	return id, nil
}

// IsAdminRequest reports whether the current request is from an admin user,
// based on the role stored in the Gin context.
func (h *BaseHandler[S, T]) IsAdminRequest(c *gin.Context) bool {
	return IsAdmin(c)
}

// BuildFiltersFromContext builds entity filters from the request query parameters.
func (h *BaseHandler[S, T]) BuildFiltersFromContext(c *gin.Context) map[string]any {
	return entigo.BuildFiltersForType[T](QueryParamsToQueryMap(c))
}

// Get handles GET requests to retrieve a single entity by ID.
func (h *BaseHandler[S, T]) Get(c *gin.Context) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.Get")
	defer span.Finish()

	id, err := h.GetIDParam(c, "id")
	if err != nil {
		h.Error(c, err)
		return
	}

	entity, err := h.service.GetByID(ctx, id)
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, "", entity)
}

// Export handles requests to export entities as a CSV file.
func (h *BaseHandler[S, T]) Export(c *gin.Context) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.Export")
	defer span.Finish()

	queryMap := QueryParamsToQueryMap(c)
	filters := entigo.BuildFiltersForType[T](queryMap)

	entities, err := h.service.List(ctx,
		entigo.WithFilter(filters),
		entigo.WithOrderFrom(queryMap),
		entigo.WithPagination(1, entigo.MaxExportSize),
	)
	if err != nil {
		h.Error(c, err)
		return
	}

	output, err := h.GetResponseData(entities)
	if err != nil {
		h.Error(c, err)
		return
	}

	// Convert entities to CSV
	csvData, err := ToCSV(output)
	if err != nil {
		h.Error(c, err)
		return
	}

	typeName := h.service.GetEntityName()
	filename := fmt.Sprintf("%s_%s.csv", strings.ToLower(typeName), time.Now().Format("20060102"))

	// Set headers for CSV download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")
	c.Header("Content-Length", fmt.Sprint(len(csvData)))

	// Write CSV data to response
	c.Data(http.StatusOK, "text/csv", csvData)
}

// List handles GET requests to list entities with pagination, filtering, and ordering.
func (h *BaseHandler[S, T]) List(c *gin.Context) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.List")
	defer span.Finish()

	queryMap := QueryParamsToQueryMap(c)
	filters := entigo.BuildFiltersForType[T](queryMap)

	totalCount, err := h.service.Count(ctx,
		entigo.WithFilter(filters),
	)
	if err != nil {
		h.Error(c, err)
		return
	}

	entities, err := h.service.List(ctx,
		entigo.WithFilter(filters),
		entigo.WithOrderFrom(queryMap),
		entigo.WithPaginationFrom(queryMap),
	)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, "", entities, "count", totalCount)
}

// ListWith handles GET requests to list entities with additional WHERE conditions.
// The args parameter is passed to entigo.WithWhereArgs for extra filtering.
func (h *BaseHandler[S, T]) ListWith(c *gin.Context, args ...any) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.ListWith")
	defer span.Finish()

	queryMap := QueryParamsToQueryMap(c)
	filters := entigo.BuildFiltersForType[T](queryMap)

	totalCount, err := h.service.Count(ctx,
		entigo.WithWhereArgs(args...),
		entigo.WithFilter(filters),
	)
	if err != nil {
		h.Error(c, err)
		return
	}

	entities, err := h.service.List(ctx,
		entigo.WithWhereArgs(args...),
		entigo.WithFilter(filters),
		entigo.WithOrderFrom(queryMap),
		entigo.WithPaginationFrom(queryMap),
	)
	if err != nil {
		h.Error(c, err)
		return
	}

	h.Success(c, "", entities, "count", totalCount)
}

// CreateOrUpdate handles requests to create or update an entity based on a key column.
// If an entity with the given key value exists, it is updated; otherwise a new one is created.
func (h *BaseHandler[S, T]) CreateOrUpdate(c *gin.Context, keyColumn string) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.CreateOrUpdate")
	defer span.Finish()

	body, err := CopyRequestBodyAsBytes(c)
	if err != nil {
		h.Error(c, NewBadRequestError(err.Error()))
		return
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		h.Error(c, NewBadRequestError(err.Error()))
		return
	}

	keyValue, exist := m[keyColumn]
	if !exist {
		h.Error(c, NewBadRequestError(fmt.Sprintf("%s is required", keyColumn)))
		return
	}

	entity, err := h.service.QueryFirst(ctx, entigo.WithWhere(keyColumn+"=?", keyValue))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.Create(c)
			return
		}
		h.Error(c, NewInternalServerError(err.Error()))
		return
	}
	c.Set("id", entity.GetID().String())

	h.Update(c)
}

// Create handles POST requests to create a new entity.
func (h *BaseHandler[S, T]) Create(c *gin.Context) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.Create")
	defer span.Finish()

	entity := h.converter.NewModelInstance()
	var err error
	if h.converter.HasCreateType() {
		input := h.converter.GenCreateRequest()
		body, _ := CopyRequestBody(c)
		if err := c.ShouldBindJSON(&input); err != nil {
			h.Logger.Error("invalid input: "+err.Error(), "error", err, "request_body", body)
			h.Error(c, NewBadRequestError("invalid input"))
			return
		}
		entity, err = h.converter.ToModel(input)

		if err != nil {
			h.Logger.Error("failed to convert input to entity", "error", err, "input", input)
			h.Error(c, NewInternalServerError(err.Error()))
			return
		}
	}

	if h.Hooks.BeforeCreate != nil {
		err = h.Hooks.BeforeCreate(c, entity)
		if err != nil {
			h.Logger.Error("error occurs before create entity", "error", err, "entity", entity)
			h.Error(c, err)
			return
		}
	}

	if err := h.service.Create(ctx, entity); err != nil {
		err = HandleDBError(err)
		h.Logger.Error("failed to create entity", "error", err, "entity", entity)
		h.Error(c, NewInternalServerError(err.Error()))
		return
	}

	if h.Hooks.AfterCreate != nil {
		err = h.Hooks.AfterCreate(c, entity)
		if err != nil {
			h.Logger.Error("error occurs after create entity", "error", err, "entity", entity)
			h.Error(c, err)
			return
		}
	}

	res, err := h.service.GetByID(ctx, entity.GetID())
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, "", res)
}

// Update handles PUT requests to update an existing entity.
func (h *BaseHandler[S, T]) Update(c *gin.Context) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.Update")
	defer span.Finish()

	id, err := h.GetID(c, "id")
	if err != nil {
		h.Error(c, NewBadRequestError(err.Error()))
		return
	}

	entity, err := h.service.GetByID(ctx, id)
	if err != nil {
		err = HandleDBError(err)
		h.Error(c, err)
		return
	}

	input := h.converter.GenUpdateRequest()
	body, _ := CopyRequestBody(c)
	if err := c.ShouldBindJSON(&input); err != nil {
		h.Logger.Error("invalid input", "error", err, "request_body", body)
		h.Error(c, NewBadRequestError("invalid input"))
		return
	}

	if err := h.converter.ToExistingModel(input, &entity); err != nil {
		h.Logger.Error("failed to convert input to entity", "error", err, "input", input)
		h.Error(c, NewInternalServerError(err.Error()))
		return
	}

	if entity.GetID() != id {
		h.Logger.Error("entity ID does not match URL ID", "entity_id", entity.GetID(), "url_id", id)
		h.Error(c, ErrBadRequest)
		return
	}

	if h.Hooks.BeforeUpdate != nil {
		err = h.Hooks.BeforeUpdate(c, entity)
		if err != nil {
			h.Logger.Error("error occurs before update entity", "error", err, "entity", entity)
			h.Error(c, err)
			return
		}
	}

	if err := h.service.Update(ctx, entity); err != nil {
		err = HandleDBError(err)
		h.Logger.Error("failed to update entity", "error", err, "entity", entity)
		h.Error(c, err)
		return
	}

	if h.Hooks.AfterUpdate != nil {
		err = h.Hooks.AfterUpdate(c, entity)
		if err != nil {
			h.Logger.Error("error occurs after update entity", "error", err, "entity", entity)
			h.Error(c, err)
			return
		}
	}

	res, err := h.service.GetByID(ctx, id)
	if err != nil {
		h.Error(c, err)
		return
	}
	h.Success(c, "", res)
}

// Patch handles PATCH requests to partially update an entity.
func (h *BaseHandler[S, T]) Patch(c *gin.Context) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.Patch")
	defer span.Finish()

	id, err := h.GetIDParam(c, "id")
	if err != nil {
		h.Error(c, NewBadRequestError("invalid ID"))
		return
	}

	input := h.converter.GenPatchRequest()
	body, _ := CopyRequestBody(c)
	if err := c.ShouldBindJSON(&input); err != nil {
		h.Logger.Error("invalid input", "error", err, "request_body", body)
		h.Error(c, NewBadRequestError("invalid input"))
		return
	}

	if h.Hooks.BeforePatch != nil {
		err = h.Hooks.BeforePatch(c, input)
		if err != nil {
			h.Logger.Error("error occurs before patch entity", "error", err, "input", input)
			h.Error(c, err)
			return
		}
	}

	if err := h.service.Patch(ctx, id, input); err != nil {
		err = HandleDBError(err)
		h.Logger.Error("failed to patch entity", "error", err, "input", input)
		h.Error(c, NewInternalServerError(err.Error()))
		return
	}

	if h.Hooks.AfterPatch != nil {
		err = h.Hooks.AfterPatch(c, input)
		if err != nil {
			h.Logger.Error("error occurs after patch entity", "error", err, "input", input)
			h.Error(c, err)
			return
		}
	}

	entity, err := h.service.GetByID(ctx, id)
	if err != nil {
		h.Logger.Error("failed to find record", "error", err, "id", id)
		h.Error(c, NewInternalServerError(err.Error()))
		return
	}
	h.Success(c, "", entity)
}

// Delete handles DELETE requests to remove an entity by ID.
func (h *BaseHandler[S, T]) Delete(c *gin.Context) {
	ctx := RequireContext(c)
	span := h.tracer.StartSpan(ctx, "BaseHandler.Delete")
	defer span.Finish()

	id, err := h.GetIDParam(c, "id")
	if err != nil {
		h.Error(c, NewBadRequestError("invalid ID"))
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		h.Logger.Error("failed to delete entity", "error", err, "id", id)
		h.Error(c, NewInternalServerError(err.Error()))
		return
	}

	h.Success(c, "", nil)
}
