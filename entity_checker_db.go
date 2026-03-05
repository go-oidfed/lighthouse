package lighthouse

import (
	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"

	oidfed "github.com/go-oidfed/lib"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// ContextualEntityChecker is an EntityChecker that requires runtime context
// to be set before checking. This is used for checkers that need access to
// storage backends or other runtime dependencies.
type ContextualEntityChecker interface {
	EntityChecker
	// SetContext sets the runtime context for this checker
	SetContext(ctx CheckerContext)
}

// CheckerContext provides runtime context for contextual entity checkers
type CheckerContext struct {
	Store         model.TrustMarkedEntitiesStorageBackend
	TrustMarkType string
}

// DBListEntityChecker checks if subject is in TrustMarkSubject table with active status.
// This checker requires SetContext to be called before Check.
type DBListEntityChecker struct {
	context *CheckerContext
}

// SetContext sets the runtime context for this checker
func (c *DBListEntityChecker) SetContext(ctx CheckerContext) {
	c.context = &ctx
}

// Check implements the EntityChecker interface
func (c *DBListEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	entityTypes []string,
) (bool, int, *oidfed.Error) {
	if c.context == nil || c.context.Store == nil {
		return false, fiber.StatusInternalServerError,
			oidfed.ErrorServerError("db_list checker not initialized with context")
	}

	status, err := c.context.Store.TrustMarkedStatus(c.context.TrustMarkType, entityConfiguration.Subject)
	if err != nil {
		return false, fiber.StatusInternalServerError, oidfed.ErrorServerError(err.Error())
	}

	switch status {
	case model.StatusActive:
		return true, 0, nil
	case model.StatusBlocked:
		return false, fiber.StatusForbidden,
			&oidfed.Error{
				Error:            "forbidden",
				ErrorDescription: "subject is blocked from this trust mark",
			}
	case model.StatusPending:
		return false, fiber.StatusAccepted,
			&oidfed.Error{
				Error:            "pending",
				ErrorDescription: "subject approval is pending",
			}
	default: // StatusInactive or unknown
		return false, fiber.StatusNotFound,
			oidfed.ErrorNotFound("subject not in active list for this trust mark type")
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (c *DBListEntityChecker) UnmarshalYAML(_ *yaml.Node) error {
	// No configuration needed for db_list checker
	return nil
}

func init() {
	RegisterEntityChecker("db_list", func() EntityChecker { return &DBListEntityChecker{} })
}
