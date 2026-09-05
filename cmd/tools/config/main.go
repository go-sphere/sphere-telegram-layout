//go:build spheretools
// +build spheretools

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-sphere/sphere-telegram-layout/internal/config"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "config",
		Short: "Config Tools",
		Long:  `Config Tools is a set of tools for config operations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	genCmd = &cobra.Command{
		Use:   "gen",
		Short: "Generate config file",
		Long:  `Generate a config file with default values.`,
	}
	testCmd = &cobra.Command{
		Use:   "test",
		Short: "Test config file format",
		Long:  `Test config file format is correct.`,
	}
)

func main() {
	Execute()
}

func init() {
	{
		flag := testCmd.Flags()
		conf := flag.String("config", "config.json", "config file path")
		testCmd.RunE = func(cmd *cobra.Command, args []string) error {
			con, err := config.NewConfig(*conf)
			if err != nil {
				return err
			}
			bytes, err := json.MarshalIndent(con, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
			return nil
		}
	}
	{

		flag := genCmd.Flags()
		output := flag.String("output", "config_gen.json", "output file path")
		database := flag.String("database", "sqlite", "database type")

		genCmd.RunE = func(*cobra.Command, []string) error {
			conf := config.NewEmptyConfig()
			switch *database {
			case "mysql":
				conf.Database.Type = "mysql"
				dsn := mysql.Config{
					User:                 "example",
					Passwd:               "password",
					Net:                  "tcp",
					Addr:                 "127.0.0.1:3306",
					DBName:               "sphere",
					Loc:                  time.Local,
					Timeout:              time.Second * 10,
					ParseTime:            true,
					AllowNativePasswords: true,
				}
				_ = dsn.Apply(mysql.Charset("utf8mb4", "utf8mb4_unicode_ci"))
				conf.Database.Path = dsn.FormatDSN()
			case "sqlite":
				conf.Database.Type = "sqlite3"
				conf.Database.Path = "file:./var/data.db?cache=shared&mode=rwc"
			}
			raw, err := json.MarshalIndent(conf, "", "  ")
			if err != nil {
				return err
			}
			return os.WriteFile(*output, raw, 0644)
		}
	}
	rootCmd.AddCommand(genCmd, testCmd)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
