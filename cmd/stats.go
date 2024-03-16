package cmd

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/fgm/drupal_redis_stats/output"
	"github.com/fgm/drupal_redis_stats/redis"
)

// statsCmd represents the stats command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "show cache stats",
	Long:  `Build a list of statistics per Drupal cache bin.`,
	RunE:  statsRun,
}

func getVerboseWriter(quiet redis.QuietValue) io.Writer {
	if quiet {
		return ioutil.Discard
	}
	return os.Stderr
}
func statsInit() {
	rootCmd.AddCommand(statsCmd)
}

func statsRun(cmd *cobra.Command, args []string) error {
	verboseWriter := getVerboseWriter(quiet)
	scanner, err := wireStatsScanner(cmd.Flags(), dsn, user, pass)
	if err != nil {
		return err
	}
	defer scanner.Close()

	s, err := scanner.Scan(verboseWriter, 0)
	if err != nil {
		return err
	}

	if jsonOutput {
		return output.JSON(os.Stdout, s)
	}
	return output.Text(os.Stdout, s)
}
