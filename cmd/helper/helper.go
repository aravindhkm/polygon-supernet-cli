package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	JSONOutputFlag  = "json"
)

type genesisParams struct {
	VaultUrl   string
	VaultToken string
}

var (
	params = &genesisParams{}
)

type OutputFormatter interface {
	// SetError sets the encountered error
	SetError(err error)

	// SetCommandResult sets the result of the command execution
	SetCommandResult(result CommandResult)

	// WriteOutput writes the previously set result / error output
	WriteOutput()

	// WriteCommandResult immediately writes the given command result without waiting for WriteOutput func call.
	WriteCommandResult(result CommandResult)

	// Write extends io.Writer interface
	Write(p []byte) (n int, err error)
}

type CommandResult interface {
	GetOutput() string
}

type commonOutputFormatter struct {
	errorOutput   error
	commandOutput CommandResult
}

type cliOutput struct {
	commonOutputFormatter
}

// newCLIOutput is the constructor of cliOutput
func newCLIOutput() *cliOutput {
	return &cliOutput{}
}

func (c *commonOutputFormatter) SetError(err error) {
	c.errorOutput = err
}

func (c *commonOutputFormatter) SetCommandResult(result CommandResult) {
	c.commandOutput = result
}

// RegisterJSONOutputFlag registers the --json output setting for all child commands
func RegisterJSONOutputFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(
		JSONOutputFlag,
		false,
		"get all outputs in json format (default false)",
	)
}

// WriteOutput implements OutputFormatter interface
func (cli *cliOutput) WriteOutput() {
	if cli.errorOutput != nil {
		_, _ = fmt.Fprintln(os.Stderr, cli.getErrorOutput())

		// return proper error exit code for cli error output
		os.Exit(1)
	}

	_, _ = fmt.Fprintln(os.Stdout, cli.getCommandOutput())
}

// WriteCommandResult implements OutputFormatter interface
func (cli *cliOutput) WriteCommandResult(result CommandResult) {
	_, _ = fmt.Fprintln(os.Stdout, result.GetOutput())
}



// WriteOutput implements OutputFormatter plus io.Writer interfaces
func (cli *cliOutput) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

func (cli *cliOutput) getErrorOutput() string {
	if cli.errorOutput == nil {
		return ""
	}

	return cli.errorOutput.Error()
}

func (cli *cliOutput) getCommandOutput() string {
	if cli.commandOutput == nil {
		return ""
	}

	return cli.commandOutput.GetOutput()
}


// cliOutput implements OutputFormatter interface by printing the output into std out in JSON format
type jsonOutput struct {
	commonOutputFormatter
}


// WriteOutput implements OutputFormatter interface
func (jo *jsonOutput) WriteOutput() {
	if jo.errorOutput != nil {
		_, _ = fmt.Fprintln(os.Stderr, jo.getErrorOutput())

		return
	}

	_, _ = fmt.Fprintln(os.Stdout, jo.getCommandOutput())
}

// WriteCommandResult implements OutputFormatter interface
func (jo *jsonOutput) WriteCommandResult(result CommandResult) {
	_, _ = fmt.Fprintln(os.Stdout, result.GetOutput())
}

// WriteOutput implements OutputFormatter plus io.Writer interfaces
func (jo *jsonOutput) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

func (jo *jsonOutput) getErrorOutput() string {
	if jo.errorOutput == nil {
		return ""
	}

	return marshalJSONToString(
		struct {
			Err string `json:"error"`
		}{
			Err: jo.errorOutput.Error(),
		},
	)
}

func (jo *jsonOutput) getCommandOutput() string {
	if jo.commandOutput == nil {
		return ""
	}

	return marshalJSONToString(jo.commandOutput)
}

func marshalJSONToString(input interface{}) string {
	bytes, err := json.Marshal(input)
	if err != nil {
		return err.Error()
	}

	return string(bytes)
}


// newJSONOutput is the constructor of jsonOutput
func newJSONOutput() *jsonOutput {
	return &jsonOutput{}
}

func InitializeOutputter(cmd *cobra.Command) OutputFormatter {
	if shouldOutputJSON(cmd) {
		return newJSONOutput()
	}

	return newCLIOutput()
}

func shouldOutputJSON(baseCmd *cobra.Command) bool {
	jsonOutputFlag := baseCmd.Flag(JSONOutputFlag)
	if jsonOutputFlag == nil {
		return false
	}

	return jsonOutputFlag.Changed
}


type GenesisResult struct {
	Message string `json:"message"`
}

func (r *GenesisResult) GetOutput() string {
	var buffer bytes.Buffer

	buffer.WriteString("\n[GENESIS SUCCESS]\n")
	buffer.WriteString(r.Message)

	return buffer.String()
}

