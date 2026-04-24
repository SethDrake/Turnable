package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	configpkg "github.com/theairblow/turnable/pkg/config"
	"github.com/theairblow/turnable/pkg/config/providers"
)

// configOptions holds CLI flags for the config subcommand
type configOptions struct {
	configPath string
	storePath  string
	routeID    string
	userUUID   string
	json       bool
}

// directConfigOptions holds CLI flags for the direct relay config subcommand
type directConfigOptions struct {
	platformId string
	callId     string
	username   string
	gateway    string
	peers      int
	json       bool
}

// newConfigCommand creates the config cobra command
func newConfigCommand() *cobra.Command {
	opts := &configOptions{}

	cmd := &cobra.Command{
		Use:   "config <route-id> <user-uuid>",
		Short: "Generates a client config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("expected exactly 2 positional arguments")
			}
			opts.routeID = args[0]
			opts.userUUID = args[1]
			return configMain(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.configPath, "config", "c", "config.json", "server config JSON file path")
	cmd.Flags().StringVarP(&opts.storePath, "store", "s", "store.json", "server user/route store JSON file path")
	cmd.Flags().BoolVarP(&opts.json, "json", "j", false, "output config in json format")
	return cmd
}

// newDirectConfigCommand creates the direct relay config cobra command
func newDirectConfigCommand() *cobra.Command {
	opts := &directConfigOptions{}

	cmd := &cobra.Command{
		Use:   "direct-config <platform-id> <call-id> <username> <gateway-addr>",
		Short: "Generates a direct relay connection client config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 4 {
				return errors.New("expected exactly 4 positional arguments")
			}
			opts.platformId = args[0]
			opts.callId = args[1]
			opts.username = args[2]
			opts.gateway = args[3]
			return directConfigMain(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.peers, "peers", "n", 1, "how many peer connections to use")
	cmd.Flags().BoolVarP(&opts.json, "json", "j", false, "output config in json format")
	return cmd
}

// configMain runs the config command
func configMain(opts *configOptions) error {
	storeData, err := os.ReadFile(opts.storePath)
	if err != nil {
		return fmt.Errorf("failed to read store json file: %w", err)
	}

	provider, err := providers.NewJSONProviderFromJSON(string(storeData))
	if err != nil {
		return fmt.Errorf("failed to parse store json file: %w", err)
	}

	configData, err := os.ReadFile(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config json file: %w", err)
	}

	serverCfg, err := configpkg.NewServerConfigFromJSON(string(configData), provider)
	if err != nil {
		return fmt.Errorf("failed to parse config json file: %w", err)
	}
	if err := serverCfg.Validate(); err != nil {
		return fmt.Errorf("failed to validate server config: %w", err)
	}

	user, err := serverCfg.GetUser(opts.userUUID)
	if err != nil {
		return fmt.Errorf("failed to resolve user: %w", err)
	}

	route, err := serverCfg.GetRoute(opts.routeID)
	if err != nil {
		return fmt.Errorf("failed to resolve route: %w", err)
	}

	clientCfg, err := serverCfg.GetClientConfig(user, route)
	if err != nil {
		return fmt.Errorf("failed to generate client config: %w", err)
	}

	if err := clientCfg.Validate(); err != nil {
		return fmt.Errorf("failed to validate client config: %w", err)
	}

	if opts.json {
		fmt.Println(clientCfg.ToJSON(false))
	} else {
		fmt.Println(clientCfg.ToURL())
	}

	return nil
}

// directConfigMain runs the direct relay config command
func directConfigMain(opts *directConfigOptions) error {
	clientCfg := configpkg.ClientConfig{
		UserUUID:   "INSECURE-DIRECT-RELAY",
		PlatformID: opts.platformId,
		CallID:     opts.callId,
		Username:   opts.username,
		Gateway:    opts.gateway,
		Peers:      opts.peers,
		Socket:     "udp",
		Type:       "direct",
		Proto:      "none",
	}

	if err := clientCfg.Validate(); err != nil {
		return fmt.Errorf("failed to validate client config: %w", err)
	}

	if opts.json {
		fmt.Println(clientCfg.ToJSON(false))
	} else {
		fmt.Println(clientCfg.ToURL())
	}

	return nil
}
