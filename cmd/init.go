// Copyright © 2021 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hyperledger/firefly-cli/internal/stacks"
	"github.com/hyperledger/firefly-cli/pkg/types"
)

var initOptions types.InitOptions
var databaseSelection string
var blockchainProviderInput string
var blockchainNodeProviderInput string
var tokenProvidersSelection []string
var promptNames bool
var releaseChannelInput string

var ffNameValidator = regexp.MustCompile(`^[0-9a-zA-Z]([0-9a-zA-Z._-]{0,62}[0-9a-zA-Z])?$`)

var stackNameInvalidRegex = regexp.MustCompile(`[^-_a-z0-9]`)

var initCmd = &cobra.Command{
	Use:   "init [stack_name] [member_count]",
	Short: "Create a new FireFly local dev stack",
	Long:  `Create a new FireFly local dev stack`,
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var stackName string
		stackManager := stacks.NewStackManager(logger)

		if err := validateDatabaseProvider(databaseSelection); err != nil {
			return err
		}
		if err := validateBlockchainProvider(blockchainProviderInput, blockchainNodeProviderInput); err != nil {
			return err
		}
		if err := validateTokensProvider(tokenProvidersSelection); err != nil {
			return err
		}
		if err := validateReleaseChannel(releaseChannelInput); err != nil {
			return err
		}

		fmt.Println("initializing new FireFly stack...")

		if len(args) > 0 {
			stackName = args[0]
			err := validateStackName(stackName)
			if err != nil {
				return err
			}
		} else {
			stackName, _ = prompt("stack name: ", validateStackName)
			fmt.Println("You selected " + stackName)
		}

		var memberCountInput string
		if len(args) > 1 {
			memberCountInput = args[1]
			if err := validateCount(memberCountInput); err != nil {
				return err
			}
		} else {
			memberCountInput, _ = prompt("number of members: ", validateCount)
		}
		memberCount, _ := strconv.Atoi(memberCountInput)

		initOptions.OrgNames = make([]string, 0, memberCount)
		initOptions.NodeNames = make([]string, 0, memberCount)
		if promptNames {
			for i := 0; i < memberCount; i++ {
				name, _ := prompt(fmt.Sprintf("name for org %d: ", i), validateFFName)
				initOptions.OrgNames = append(initOptions.OrgNames, name)
				name, _ = prompt(fmt.Sprintf("name for node %d: ", i), validateFFName)
				initOptions.NodeNames = append(initOptions.NodeNames, name)
			}
		} else {
			for i := 0; i < memberCount; i++ {
				initOptions.OrgNames = append(initOptions.OrgNames, fmt.Sprintf("org_%d", i))
				initOptions.NodeNames = append(initOptions.NodeNames, fmt.Sprintf("node_%d", i))
			}
		}

		initOptions.Verbose = verbose
		initOptions.BlockchainProvider, initOptions.BlockchainNodeProvider, _ = types.BlockchainFromStrings(blockchainProviderInput, blockchainNodeProviderInput)
		initOptions.DatabaseSelection, _ = types.DatabaseSelectionFromString(databaseSelection)
		initOptions.TokenProviders, _ = types.TokenProvidersFromStrings(tokenProvidersSelection)
		initOptions.ReleaseChannel, _ = types.ReleaseChannelSelectionFromString(releaseChannelInput)

		if err := stackManager.InitStack(stackName, memberCount, &initOptions); err != nil {
			return err
		}

		fmt.Printf("Stack '%s' created!\nTo start your new stack run:\n\n%s start %s\n", stackName, rootCmd.Use, stackName)
		fmt.Printf("\nYour docker compose file for this stack can be found at: %s\n\n", filepath.Join(stackManager.Stack.StackDir, "docker-compose.yml"))
		return nil
	},
}

func validateStackName(stackName string) error {
	if strings.TrimSpace(stackName) == "" {
		return errors.New("stack name must not be empty")
	}

	if stackNameInvalidRegex.Find([]byte(stackName)) != nil {
		return fmt.Errorf("stack name may not contain any character matching the regex: %s", stackNameInvalidRegex)
	}

	if exists, err := stacks.CheckExists(stackName); exists {
		return fmt.Errorf("stack '%s' already exists", stackName)
	} else {
		return err
	}
}

func validateCount(input string) error {
	if i, err := strconv.Atoi(input); err != nil {
		return errors.New("invalid number")
	} else if i <= 0 {
		return errors.New("number of members must be greater than zero")
	} else if initOptions.ExternalProcesses >= i {
		return errors.New("number of external processes should not be equal to or greater than the number of members in the network - at least one FireFly core container must exist to be able to extract and deploy smart contracts")
	}
	return nil
}

func validateFFName(input string) error {
	if !ffNameValidator.MatchString(input) {
		return fmt.Errorf("name must be 1-64 characters, including alphanumerics (a-zA-Z0-9), dot (.), dash (-) and underscore (_), and must start/end in an alphanumeric")
	}
	return nil
}

func validateDatabaseProvider(input string) error {
	_, err := types.DatabaseSelectionFromString(input)
	if err != nil {
		return err
	}
	return nil
}

func validateBlockchainProvider(providerString, nodeString string) error {
	blockchainSelection, _, err := types.BlockchainFromStrings(providerString, nodeString)
	if err != nil {
		return err
	}

	if blockchainSelection == types.Corda {
		return errors.New("support for corda is coming soon")
	}

	// TODO: When we get tokens on Fabric this should change
	if blockchainSelection == types.HyperledgerFabric {
		tokenProvidersSelection = []string{}
	}

	return nil
}

func validateTokensProvider(input []string) error {
	_, err := types.TokenProvidersFromStrings(input)
	if err != nil {
		return err
	}
	return nil
}

func validateReleaseChannel(input string) error {
	_, err := types.ReleaseChannelSelectionFromString(input)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	initCmd.Flags().IntVarP(&initOptions.FireFlyBasePort, "firefly-base-port", "p", 5000, "Mapped port base of FireFly core API (1 added for each member)")
	initCmd.Flags().IntVarP(&initOptions.ServicesBasePort, "services-base-port", "s", 5100, "Mapped port base of services (100 added for each member)")
	initCmd.Flags().StringVarP(&databaseSelection, "database", "d", "sqlite3", fmt.Sprintf("Database type to use. Options are: %v", types.DBSelectionStrings))
	initCmd.Flags().StringVarP(&blockchainProviderInput, "blockchain-provider", "b", "ethereum", fmt.Sprintf("Blockchain to use. Options are: %v", types.BlockchainProviderStrings))
	initCmd.Flags().StringVarP(&blockchainNodeProviderInput, "blockchain-node", "n", "geth", fmt.Sprintf("Blockchain node type to use. Options are: %v", types.BlockchainNodeProviderStrings))
	initCmd.Flags().StringArrayVarP(&tokenProvidersSelection, "token-providers", "t", []string{"erc20_erc721"}, fmt.Sprintf("Token providers to use. Options are: %v", types.ValidTokenProviders))
	initCmd.Flags().IntVarP(&initOptions.ExternalProcesses, "external", "e", 0, "Manage a number of FireFly core processes outside of the docker-compose stack - useful for development and debugging")
	initCmd.Flags().StringVarP(&initOptions.FireFlyVersion, "release", "r", "latest", "Select the FireFly release version to use")
	initCmd.Flags().StringVarP(&initOptions.ManifestPath, "manifest", "m", "", "Path to a manifest.json file containing the versions of each FireFly microservice to use. Overrides the --release flag.")
	initCmd.Flags().BoolVar(&promptNames, "prompt-names", false, "Prompt for org and node names instead of using the defaults")
	initCmd.Flags().BoolVar(&initOptions.PrometheusEnabled, "prometheus-enabled", false, "Enables Prometheus metrics exposition and aggregation to a shared Prometheus server")
	initCmd.Flags().BoolVar(&initOptions.SandboxEnabled, "sandbox-enabled", true, "Enables the FireFly Sandbox to be started with your FireFly stack")
	initCmd.Flags().BoolVar(&initOptions.FFTMEnabled, "fftm-enabled", false, "Starts a FireFly Transaction Manager runtime for each node")
	initCmd.Flags().IntVar(&initOptions.PrometheusPort, "prometheus-port", 9090, "Port for the shared Prometheus server")
	initCmd.Flags().StringVarP(&initOptions.ExtraCoreConfigPath, "core-config", "", "", "The path to a yaml file containing extra config for FireFly Core")
	initCmd.Flags().StringVarP(&initOptions.ExtraEthconnectConfigPath, "ethconnect-config", "", "", "The path to a yaml file containing extra config for Ethconnect")
	initCmd.Flags().StringVarP(&initOptions.ExtraFFTMConfigPath, "fftm-config", "", "", "The path to a yaml file containing extra config for FireFly Transaction Manager")
	initCmd.Flags().IntVarP(&initOptions.BlockPeriod, "block-period", "", -1, "Block period in seconds. Default is variable based on selected blockchain provider.")
	initCmd.Flags().StringVarP(&initOptions.ContractAddress, "contract-address", "", "", "Do not automatically deploy a contract, instead use a pre-configured address")
	initCmd.Flags().StringVarP(&initOptions.RemoteNodeURL, "remote-node-url", "", "", "For cases where the node is pre-existing and running remotely")
	initCmd.Flags().Int64VarP(&initOptions.ChainID, "chain-id", "", 2021, "The chain ID (Ethereum only) - also used as the network ID")
	initCmd.Flags().IntVarP(&initOptions.RequestTimeout, "request-timeout", "", 0, "Custom request timeout (in seconds) - useful for registration to public chains")
	initCmd.Flags().StringVarP(&releaseChannelInput, "channel", "", "stable", fmt.Sprintf("Select the FireFly release channel to use. Options are: %v", types.ReleaseChannelSelectionStrings))
	initCmd.Flags().BoolVarP(&initOptions.MultipartyEnabled, "multiparty", "", true, "Enable or disable multiparty mode. Set to 'false' to use gateway mode.")

	rootCmd.AddCommand(initCmd)
}
