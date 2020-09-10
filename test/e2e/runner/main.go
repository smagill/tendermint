package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/libs/log"
)

var (
	logger  = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	rootCmd = &cobra.Command{
		Use:   "runner",
		Short: "End-to-end test runner",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}
			dir, err := cmd.Flags().GetString("dir")
			if err != nil {
				return err
			}
			if dir == "" {
				dir = filepath.Dir(file)
			}

			manifest, err := LoadManifest(file)
			if err != nil {
				return err
			}
			testnet, err := NewTestnet(manifest)
			if err != nil {
				return err
			}
			err = testnet.WriteConfig(dir)
			if err != nil {
				return err
			}
			logger.Info(fmt.Sprintf("Generated testnet files in %q", dir))
			return nil
		},
	}
)

func init() {
	rootCmd.Flags().StringP("file", "f", "", "Testnet TOML manifest")
	_ = rootCmd.MarkFlagRequired("file")
	rootCmd.Flags().StringP("dir", "d", "", "Directory to use for testnet data (defaults to manifest dir)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}