package generator

import (
	"fmt"
	"os"
)

type options struct {
	dataSourceName string
	tableNames     []string
	forceCases     []string
}

func printUsageAndExit(exampleDataSourceName string) {
	cmd := os.Args[0]
	_, _ = fmt.Fprintf(os.Stderr, `Usage:
	%s -o outpath -d datasource [-t table1,table2,...] [-forcecases ID,IDs,HTML] dataSourceName
Example:
	%s "%s"
`, cmd, cmd, fmt.Sprintf("-o ./ -d %s", exampleDataSourceName))
	os.Exit(1)
}
