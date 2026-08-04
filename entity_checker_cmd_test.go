package lighthouse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	oidfed "github.com/go-oidfed/lib"
)

func testEntityStatement(sub string) *oidfed.EntityStatement {
	return &oidfed.EntityStatement{
		EntityStatementPayload: oidfed.EntityStatementPayload{
			Issuer:  sub,
			Subject: sub,
		},
	}
}

func TestCmdEntityChecker_AllowOnExitZero(t *testing.T) {
	c := &CmdEntityChecker{Path: "sh", Args: []string{"-c", "exit 0"}}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), []string{"openid_provider"})
	assert.True(t, ok)
	assert.Equal(t, 0, code)
	assert.Nil(t, errResp)
}

func TestCmdEntityChecker_DenyOnNonZeroExit(t *testing.T) {
	c := &CmdEntityChecker{
		Path: "sh",
		Args: []string{"-c", "echo 'not allowed' >&2; exit 1"},
	}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), []string{"openid_provider"})
	assert.False(t, ok)
	assert.Equal(t, 403, code)
	require.NotNil(t, errResp)
	assert.Equal(t, "forbidden", errResp.Error)
	assert.Equal(t, "not allowed", errResp.ErrorDescription)
}

func TestCmdEntityChecker_DenyEmptyStderr(t *testing.T) {
	c := &CmdEntityChecker{Path: "sh", Args: []string{"-c", "exit 2"}}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, 403, code)
	require.NotNil(t, errResp)
	assert.Equal(t, "entity check failed", errResp.ErrorDescription)
}

func TestCmdEntityChecker_EntityIDEnvVar(t *testing.T) {
	c := &CmdEntityChecker{
		Path: "sh",
		Args: []string{"-c", "test \"$ENTITY_ID\" = \"https://op.example.org\""},
	}
	ok, _, _ := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.True(t, ok, "command should succeed when ENTITY_ID matches")
}

func TestCmdEntityChecker_EntityTypesEnvVar(t *testing.T) {
	c := &CmdEntityChecker{
		Path: "sh",
		Args: []string{"-c", "test \"$ENTITY_TYPES\" = \"openid_provider,trust_anchor\""},
	}
	ok, _, _ := c.Check(
		testEntityStatement("https://op.example.org"),
		[]string{"openid_provider", "trust_anchor"},
	)
	assert.True(t, ok, "command should succeed when ENTITY_TYPES matches")
}

func TestCmdEntityChecker_ExtraEnvVar(t *testing.T) {
	c := &CmdEntityChecker{
		Path: "sh",
		Args: []string{"-c", "test \"$MY_VAR\" = \"hello\""},
		Env:  []string{"MY_VAR=hello"},
	}
	ok, _, _ := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.True(t, ok, "command should see extra env var")
}

func TestCmdEntityChecker_StdinReceivesEntityConfiguration(t *testing.T) {
	// The command reads stdin and exits 0 only if it contains the expected
	// entity configuration JSON (the "sub" field).
	c := &CmdEntityChecker{
		Path: "sh",
		Args: []string{"-c", "grep -q '\"sub\":\"https://op.example.org\"'"},
	}
	ok, _, _ := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.True(t, ok, "stdin should contain the entity configuration payload")
}

func TestCmdEntityChecker_StdinContent(t *testing.T) {
	// Verify the full stdin payload matches the marshalled EntityStatementPayload.
	es := testEntityStatement("https://op.example.org")
	expected, _ := json.Marshal(es.EntityStatementPayload)

	tmp := filepath.Join(t.TempDir(), "stdin.json")
	c := &CmdEntityChecker{
		Path: "sh",
		Args: []string{"-c", "cat > " + tmp},
	}
	c.Check(es, nil)

	got, err := os.ReadFile(tmp)
	require.NoError(t, err)
	assert.JSONEq(t, string(expected), string(got))
}

func TestCmdEntityChecker_Timeout(t *testing.T) {
	c := &CmdEntityChecker{
		Path:    "sh",
		Args:    []string{"-c", "sleep 10"},
		Timeout: 1,
	}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, 500, code)
	require.NotNil(t, errResp)
	assert.Contains(t, errResp.ErrorDescription, "timed out")
}

func TestCmdEntityChecker_MissingPath(t *testing.T) {
	c := &CmdEntityChecker{}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, 500, code)
	require.NotNil(t, errResp)
	assert.Contains(t, errResp.ErrorDescription, "path is required")
}

func TestCmdEntityChecker_CommandNotFound(t *testing.T) {
	c := &CmdEntityChecker{Path: "/nonexistent/command/xyz"}
	ok, code, errResp := c.Check(testEntityStatement("https://op.example.org"), nil)
	assert.False(t, ok)
	assert.Equal(t, 500, code)
	require.NotNil(t, errResp)
	assert.Contains(t, errResp.ErrorDescription, "could not run command")
}

func TestCmdEntityChecker_UnmarshalYAML(t *testing.T) {
	yamlStr := `
path: /usr/local/bin/check.sh
args:
  - --strict
env:
  - MODE=prod
timeout: 15
`
	var c CmdEntityChecker
	err := yaml.Unmarshal([]byte(yamlStr), &c)
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/check.sh", c.Path)
	assert.Equal(t, []string{"--strict"}, c.Args)
	assert.Equal(t, []string{"MODE=prod"}, c.Env)
	assert.Equal(t, 15, c.Timeout)
}

func TestCmdEntityChecker_Registered(t *testing.T) {
	ctor, ok := entityCheckerRegistry["cmd"]
	require.True(t, ok, "cmd checker should be registered")
	assert.NotNil(t, ctor())
}
