package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ficoto/sqlingo/generator"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	warningLines := []string{
		"\u001b[31mThis command is deprecated. Please install the new generator with the corresponding driver:",
		"go get -u github.com/lqs/sqlingo/sqlingo-gen-mysql",
		"go get -u github.com/lqs/sqlingo/sqlingo-gen-sqlite3",
		"go get -u github.com/lqs/sqlingo/sqlingo-gen-postgres",
		"\u001b[0m",
	}
	_, _ = fmt.Fprintln(os.Stderr, strings.Join(warningLines, "\n"))
	err := generator.Generate("mysql", "username:password@tcp(hostname:3306)/database")
	if err != nil {
		panic(err)
	}
}
