package cmd

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/fgm/drupal_redis_stats/redis"
)

var (
	cfgFile    string
	dsn        redis.DSNValue = "redis://localhost:6379/0"
	jsonOutput redis.JSONValue
	quiet      redis.QuietValue
	user       redis.UserValue
	pass       redis.PassValue
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:               path.Base(os.Args[0]),
	Short:             "Drupal Redis cache control",
	Long:              `Commands helping with the use of the Redis cache in Drupal 8/9.`,
	RunE:              statsRun,
	PersistentPreRunE: rootPreRunE,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootInit()
	statsInit()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func rootInit() {
	cobra.OnInitialize(initConfig)
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.drupal_redis_stats.yaml)")
	pf.Var(&dsn, "dsn", "Can include user and password, per https://www.iana.org/assignments/uri-schemes/prov/redis")
	pf.VarP(&user, "user", "u", "user name if Redis is configured with ACL. Overrides the DSN user.")
	pf.VarP(&pass, "pass", "p", "Password. If it is empty it's asked from the tty. Overrides the DSN password.")
	pf.VarP(&jsonOutput, "json", "j", "Use JSON output.")
	pf.VarP(&quiet, "quiet", "q", "Do not display scan progress")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".drupal_redis_stats" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".drupal_redis_stats")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func rootPreRunE(cmd *cobra.Command, _ []string) error {
	var err error
	user, pass, err = getCredentials(cmd.Flags(), os.Stdout, dsn, user, pass)
	if err != nil {
		return fmt.Errorf("failed obtaining user/pass: %v", err)
	}
	return nil
}
