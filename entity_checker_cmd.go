package lighthouse

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	oidfed "github.com/go-oidfed/lib"
)

// CmdEntityChecker runs an external command to decide whether an entity
// satisfies the requirements. The entity's Entity Configuration payload is
// written to the command's stdin as JSON, and the entity ID and types are
// exposed through the ENTITY_ID and ENTITY_TYPES environment variables.
//
// The command's exit code determines the result: exit 0 allows the entity,
// any non-zero exit denies it. stderr (truncated) is used as the error
// description on denial.
type CmdEntityChecker struct {
	Path    string   `yaml:"path" json:"path"`
	Args    []string `yaml:"args" json:"args"`
	Env     []string `yaml:"env" json:"env"`
	Timeout int      `yaml:"timeout" json:"timeout"`
}

// Check implements the EntityChecker interface
func (c *CmdEntityChecker) Check(
	entityConfiguration *oidfed.EntityStatement,
	entityTypes []string,
) (bool, int, *oidfed.Error) {
	if c.Path == "" {
		return false, fiber.StatusInternalServerError,
			oidfed.ErrorServerError("cmd checker: path is required")
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Path, c.Args...)

	// Build environment: inherit current env, add entity vars, then user env.
	cmd.Env = append(
		os.Environ(),
		"ENTITY_ID="+entityConfiguration.Subject,
		"ENTITY_TYPES="+strings.Join(entityTypes, ","),
	)
	if len(c.Env) > 0 {
		cmd.Env = append(cmd.Env, c.Env...)
	}

	// Write the Entity Configuration payload to stdin.
	payloadBytes, err := json.Marshal(entityConfiguration.EntityStatementPayload)
	if err != nil {
		return false, fiber.StatusInternalServerError,
			oidfed.ErrorServerError("cmd checker: could not marshal entity configuration: " + err.Error())
	}
	cmd.Stdin = bytes.NewReader(payloadBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// WaitDelay ensures cmd.Wait returns promptly after a timeout even if the
	// command spawned child processes that still hold the stdout/stderr pipes.
	cmd.WaitDelay = time.Duration(timeout) * time.Second

	if err = cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			log.Warn().Err(err).
				Str("entity_id", entityConfiguration.Subject).
				Str("cmd", c.Path).
				Msg("cmd checker: command timed out")
			return false, fiber.StatusInternalServerError,
				oidfed.ErrorServerError("cmd checker: command timed out")
		}
		// A non-zero exit code denies the entity; stderr is the description.
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			desc := strings.TrimSpace(stderr.String())
			if desc == "" {
				desc = "entity check failed"
			}
			if len(desc) > 500 {
				desc = desc[:500]
			}
			log.Debug().
				Str("entity_id", entityConfiguration.Subject).
				Str("cmd", c.Path).
				Str("stderr", desc).
				Msg("cmd checker: command denied entity")
			return false, fiber.StatusForbidden, &oidfed.Error{
				Error:            "forbidden",
				ErrorDescription: desc,
			}
		}
		// Could not start the command (not found, not executable, etc.).
		log.Warn().Err(err).
			Str("entity_id", entityConfiguration.Subject).
			Str("cmd", c.Path).
			Msg("cmd checker: could not run command")
		return false, fiber.StatusInternalServerError,
			oidfed.ErrorServerError("cmd checker: could not run command: " + err.Error())
	}

	log.Debug().
		Str("entity_id", entityConfiguration.Subject).
		Str("cmd", c.Path).
		Msg("cmd checker: command allowed entity")
	return true, 0, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (c *CmdEntityChecker) UnmarshalYAML(node *yaml.Node) error {
	type Alias CmdEntityChecker
	var alias Alias
	if err := node.Decode(&alias); err != nil {
		return errors.WithStack(err)
	}
	*c = CmdEntityChecker(alias)
	return nil
}

func init() {
	RegisterEntityChecker("cmd", func() EntityChecker { return &CmdEntityChecker{} })
}
