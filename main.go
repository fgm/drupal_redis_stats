/*
This simple CLI application scans a Redis database for Drupal 8/9 cache content.

It then return statistics about that instance, in plain text or JSON, and allows
clearing that cache.
*/
package main

import (
	"github.com/fgm/drupal_redis_stats/cmd"
)

func main() {
	cmd.Execute()
}
