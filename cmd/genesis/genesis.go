package genesis

import (
	"cli/cmd/helper"
	"errors"
	"strings"
	"time"
	"fmt"

	"cli/cmd/chain"

	"github.com/briandowns/spinner"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

type genesisParams struct {
	Name            string
	TotalNode       string
	GasLimit        string
	EpochSize       string
	NodePremineFund string
	Premine         chain.PremineAllo
	VaultUrl        string
	VaultToken      string
}

var (
	params = &genesisParams{}
)

type PremineAllo []chain.PremineAllo

var premine PremineAllo

func (p *PremineAllo) String() string {
	var account []string
	for _, premine := range *p {
		account = append(account, premine.Account)
	}
	return strings.Join(account, ", ")
}

func (p *PremineAllo) Type() string {
	return "PremineAllo"
}

func (p *PremineAllo) Set(value string) error {
	premineAlloData := strings.Split(value, ",")

	var premineAllo []chain.PremineAllo
	for _, data := range premineAlloData {
		result := strings.Split(data, ":")

		if len(result) != 2 {
			return fmt.Errorf("invalid premine format: %s", data)
		}

		account := result[0]
		amount := result[1]

		premineAllo = append(premineAllo, chain.PremineAllo{Account: account, Amount: amount})
	}

	*p = premineAllo
	return nil
}

const (
	VaultToken      = "valut-token"
	VaultUrl        = "valut-url"
	Name            = "name"
	TotalNode       = "totalNode"
	GasLimit        = "gasLimit"
	EpochSize       = "epochSize"
	NodePremineFund = "nodePremineFund"
	Premine         = "premine"
)

func (p *genesisParams) getResult() helper.CommandResult {
	return &helper.GenesisResult{
		Message: fmt.Sprintf("\nGenesis executed successfully \n"),
	}
}

func GetCommand() *cobra.Command {
	genesisCmd := &cobra.Command{
		Use:     "genesis",
		Short:   "Generates the genesis configuration file with the passed in parameters",
		PreRunE: preRunCommand,
		Run:     runCommand,
	}

	setFlags(genesisCmd)

	return genesisCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&params.Name,
		Name,
		"", // example polyedge-local
		"the name for the network",
	)

	cmd.Flags().StringVar(
		&params.TotalNode,
		TotalNode,
		"", // example 4
		"number of total validator node",
	)

	cmd.Flags().StringVar(
		&params.GasLimit,
		GasLimit,
		"10000000",
		"the maximum amount of gas used by all transactions in a block",
	)

	cmd.Flags().StringVar(
		&params.EpochSize,
		EpochSize,
		"10",
		"the epoch size for the network",
	)

	cmd.Flags().StringVar(
		&params.NodePremineFund,
		NodePremineFund,
		"", // 1000000000000000
		"the premine amount for the validator accounts",
	)

	cmd.Flags().VarP(
		&premine,
		Premine,
		"p",
		"the premined accounts and balances (format: [<address>:<balance>]).",
	)

	cmd.Flags().StringVar(
		&params.VaultToken,
		VaultToken,
		"",
		"secure valut path",
	)

	cmd.Flags().StringVar(
		&params.VaultUrl,
		VaultUrl,
		"",
		"secure valut path",
	)
}

func validateFlags() error {
	if params.Name == "" {
		return errors.New("Chain name is required")
	}

	if params.TotalNode == "" {
		return errors.New("Total node field is required")
	}

	if params.NodePremineFund == "" {
		return errors.New("Node-Premine amount is required")
	}

	return nil
}

func preRunCommand(cmd *cobra.Command, _ []string) error {
	if err := validateFlags(); err != nil {
		return err
	}

	return nil
}

func runCommand(cmd *cobra.Command, _ []string) {
	outputter := helper.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	var err error

	s := spinner.New(spinner.CharSets[14], 110*time.Millisecond, spinner.WithColor("cyan"))

	req := chain.ConfigRequest{
		Name:              cmd.Flag(Name).Value.String(),
		NumOfNodes:        cmd.Flag(TotalNode).Value.String(),
		GasLimit:          cmd.Flag(GasLimit).Value.String(),
		EpochSize:         cmd.Flag(EpochSize).Value.String(),
		NodePremineAmount: cmd.Flag(NodePremineFund).Value.String(),
		Premine:           premine,
	}

	for _, person := range premine {

		isValid := common.IsHexAddress(person.Account)
		if !isValid {
			outputter.SetError(errors.New(fmt.Sprintf("The Ethereum address %s is invalid.\n", person.Account)))
			return
		}
	}

	fmt.Println("\n ")
	s.Suffix = " Running..."
	s.Start()

	namespace, response, err := chain.CreateConfigMap(req)
	if err != nil {
		emitCmd(s, "Initialize-Crypto is failed", false)
		outputter.SetError(err)
		return
	} else {
		emitCmd(s, response, true)
	}

	result, err := chain.CreateNodeConfigMap(namespace)
	if err != nil {
		emitCmd(s, "Validator node config is failed", false)
		outputter.SetError(err)
		return
	} else {
		emitCmd(s, result, true)
	}

	result, err = chain.CreateStorageClassAndPVC(namespace)
	if err != nil {
		emitCmd(s, "PersistentVolumeClaim config is failed", false)
		outputter.SetError(err)
		return
	} else {
		emitCmd(s, result, true)
	}

	result, err = chain.CreateStateFulSet(namespace)
	if err != nil {
		emitCmd(s, "Statefulset config is failed", false)
		outputter.SetError(err)
		return
	} else {
		emitCmd(s, result, true)
	}

	result, err = chain.CreateLoadBalancer(namespace)
	if err != nil {
		emitCmd(s, "LoadBalancer config is failed", false)
		outputter.SetError(err)
		return
	} else {
		emitCmd(s, result, true)
	}

	fmt.Printf("\nyour stake id is %s \n", namespace)

	outputter.SetCommandResult(params.getResult())
}

func emitCmd(s *spinner.Spinner, value string, status bool) {
	time.Sleep(2 * time.Second)

	if status {
		s.Stop()
		fmt.Printf("\x1b[32m✓\x1b[0m %s\n", value)
		s.Start()
	} else {
		s.Stop()
		fmt.Printf("\x1b[31m✗\x1b[0m %s\n", value)
		s.Start()
	}
}
