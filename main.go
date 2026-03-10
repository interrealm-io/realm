package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/interrealm-io/realm/internal/config"
	"github.com/interrealm-io/realm/internal/identity"
	"github.com/interrealm-io/realm/internal/server"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "realm",
	Short: "realm — the InterRealm container runtime",
	Long: `realm is the reference runtime for the InterRealm protocol.

It runs a realm as a live process: loading its identity, exposing
capability tool endpoints over HTTP, and optionally registering
its address on the realmnet distributed ledger.

Configure your realm in realm.yaml, then:

  realm start          start the runtime
  realm keygen         generate a keypair
  realm status         show current realm status
  realm capabilities   list exposed tool endpoints`,
}

func main() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "realm.yaml", "path to realm.yaml")

	rootCmd.AddCommand(
		startCmd,
		keygenCmd,
		statusCmd,
		capabilitiesCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// --- start ---

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the realm runtime",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		id, err := identity.Load(cfg.Realm.ID, cfg.Realm.Keyfile)
		if err != nil {
			return fmt.Errorf("load identity: %w", err)
		}

		log.Printf("[realm] starting %s (%s)", cfg.Realm.ID, cfg.Realm.Name)
		log.Printf("[realm] mode: %s", cfg.Realm.Mode)

		if cfg.Realm.Mode == config.ModePublic {
			log.Printf("[realm] will register on realmnet at startup")
			// TODO: call realmnet registration
		}

		srv := server.New(cfg, id)
		return srv.Start()
	},
}

// --- keygen ---

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a keypair for this realm",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		outDir, _ := cmd.Flags().GetString("out")
		fmt.Printf("Generating keypair for realm: %s\n", cfg.Realm.ID)

		_, err = identity.Generate(cfg.Realm.ID, outDir)
		if err != nil {
			return err
		}

		fmt.Println("Done. Update realm.yaml keyfile path if needed.")
		return nil
	},
}

func init() {
	keygenCmd.Flags().String("out", "./keys/", "Output directory for key files")
}

// --- status ---

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current realm status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Printf("Realm ID:   %s\n", cfg.Realm.ID)
		fmt.Printf("Name:       %s\n", cfg.Realm.Name)
		fmt.Printf("Mode:       %s\n", cfg.Realm.Mode)
		fmt.Printf("Port:       %d\n", cfg.Network.Port)
		if cfg.Network.Endpoint != "" {
			fmt.Printf("Endpoint:   %s\n", cfg.Network.Endpoint)
		}
		fmt.Printf("Keyfile:    %s\n", cfg.Realm.Keyfile)
		fmt.Printf("Tools:      %d configured\n", len(cfg.Capabilities.Tools))
		return nil
	},
}

// --- capabilities ---

var capabilitiesCmd = &cobra.Command{
	Use:   "capabilities",
	Short: "List capability tool endpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Printf("Capabilities for %s:\n\n", cfg.Realm.ID)
		for _, t := range cfg.Capabilities.Tools {
			public := "  [auth required]"
			if t.Public {
				public = "  [public]"
			}
			fmt.Printf("  %-8s %s%s%s\n    %s\n\n",
				t.Method,
				cfg.Capabilities.BasePath,
				t.Path,
				public,
				t.Description,
			)
		}
		return nil
	},
}
