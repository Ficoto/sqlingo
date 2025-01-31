package generator

import (
	"bytes"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	sqlingoGeneratorVersion = 2
)

type schemaFetcher interface {
	GetDatabaseName() (dbName string, err error)
	GetTableNames() (tableNames []string, err error)
	GetFieldDescriptors(tableName string) ([]fieldDescriptor, error)
	QuoteIdentifier(identifier string) string
}

type fieldDescriptor struct {
	Name      string
	Type      string
	Size      int
	Unsigned  bool
	AllowNull bool
	Comment   string
}

func convertToExportedIdentifier(s string, forceCases []string) string {
	var words []string
	nextCharShouldBeUpperCase := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if nextCharShouldBeUpperCase {
				words = append(words, "")
				words[len(words)-1] += string(unicode.ToUpper(r))
				nextCharShouldBeUpperCase = false
			} else {
				words[len(words)-1] += string(r)
			}
		} else {
			nextCharShouldBeUpperCase = true
		}
	}
	result := ""
	for _, word := range words {
		for _, caseWord := range forceCases {
			if strings.EqualFold(word, caseWord) {
				word = caseWord
				break
			}
		}
		result += word
	}
	var firstRune rune
	for _, r := range result {
		firstRune = r
		break
	}
	if result == "" || !unicode.IsUpper(firstRune) {
		result = "E" + result
	}
	return result
}

func getType(fieldDescriptor fieldDescriptor) (goType string, fieldClass string, err error) {
	switch strings.ToLower(fieldDescriptor.Type) {
	case "tinyint":
		goType = "int8"
		fieldClass = "NumberField"
	case "smallint":
		goType = "int16"
		fieldClass = "NumberField"
	case "int", "mediumint":
		goType = "int32"
		fieldClass = "NumberField"
	case "bigint", "integer":
		goType = "int64"
		fieldClass = "NumberField"
	case "float", "double", "decimal", "real":
		goType = "float64"
		fieldClass = "NumberField"
	case "char", "varchar", "text", "tinytext", "mediumtext", "longtext", "enum", "datetime", "date", "time", "timestamp", "json", "numeric", "character varying":
		goType = "string"
		fieldClass = "StringField"
	case "binary", "varbinary", "blob", "tinyblob", "mediumblob", "longblob":
		// TODO: use []byte ?
		goType = "string"
		fieldClass = "StringField"
	case "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring", "multipolygon", "geometrycollection":
		goType = "sqlingo.WellKnownBinary"
		fieldClass = "WellKnownBinaryField"
	case "bit":
		if fieldDescriptor.Size == 1 {
			goType = "bool"
			fieldClass = "BooleanField"
		} else {
			goType = "string"
			fieldClass = "StringField"
		}
	default:
		err = fmt.Errorf("unknown field type %s", fieldDescriptor.Type)
		return
	}
	if fieldDescriptor.Unsigned && strings.HasPrefix(goType, "int") {
		goType = "u" + goType
	}
	if fieldDescriptor.AllowNull {
		goType = "*" + goType
	}
	return
}

func getSchemaFetcherFactory(driverName string) func(db *sql.DB) schemaFetcher {
	switch driverName {
	case "mysql":
		return newMySQLSchemaFetcher
	case "sqlite3":
		return newSQLite3SchemaFetcher
	case "postgres":
		return newPostgresSchemaFetcher
	default:
		_, _ = fmt.Fprintln(os.Stderr, "unsupported driver "+driverName)
		os.Exit(2)
		return nil
	}
}

var nonIdentifierRegexp = regexp.MustCompile(`\W`)

func ensureIdentifier(name string) string {
	result := nonIdentifierRegexp.ReplaceAllString(name, "_")
	if result == "" || (result[0] >= '0' && result[0] <= '9') {
		result = "_" + result
	}
	return result
}

func newBuffWithBaseHeader(packageName string) *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteString("// This file is generated by sqlingo (https://github.com/lqs/sqlingo)\n")
	buf.WriteString("// DO NOT EDIT.\n\n")
	buf.WriteString("package " + ensureIdentifier(packageName) + "_dsl\n")
	buf.WriteString("import \"github.com/lqs/sqlingo\"\n\n")
	return &buf
}

var (
	outputPath         = flag.String("o", "", "file output path")
	databaseConnection = flag.String("dbc", "", "database connection")
	tables             = flag.String("t", "", "-t table1,table2,...")
	forcecases         = flag.String("forcecases", "", "-forcecases ID,IDs,HTML")
)

// Generate generates code for the given driverName.
func Generate(driverName string, exampleDataSourceName string) error {
	flag.Parse()
	if len(*outputPath) == 0 {
		printUsageAndExit(exampleDataSourceName)
	}
	if len(*databaseConnection) == 0 {
		printUsageAndExit(exampleDataSourceName)
	}
	var options options
	options.dataSourceName = *databaseConnection
	if len(*tables) != 0 {
		options.tableNames = strings.Split(*tables, ",")
	}
	if len(*forcecases) != 0 {
		options.forceCases = strings.Split(*forcecases, ",")
	}

	db, err := sql.Open(driverName, options.dataSourceName)
	if err != nil {
		return err
	}

	schemaFetcherFactory := getSchemaFetcherFactory(driverName)
	schemaFetcher := schemaFetcherFactory(db)

	dbName, err := schemaFetcher.GetDatabaseName()
	if err != nil {
		return err
	}

	if dbName == "" {
		return errors.New("no database selected")
	}

	var buf = newBuffWithBaseHeader(dbName)

	buf.WriteString("type sqlingoRuntimeAndGeneratorVersionsShouldBeTheSame uint32\n\n")

	sqlingoGeneratorVersionString := strconv.Itoa(sqlingoGeneratorVersion)
	buf.WriteString("const _ = sqlingoRuntimeAndGeneratorVersionsShouldBeTheSame(sqlingo.SqlingoRuntimeVersion - " + sqlingoGeneratorVersionString + ")\n")
	buf.WriteString("const _ = sqlingoRuntimeAndGeneratorVersionsShouldBeTheSame(" + sqlingoGeneratorVersionString + " - sqlingo.SqlingoRuntimeVersion)\n\n")

	buf.WriteString("type table interface {\n")
	buf.WriteString("\tsqlingo.Table\n")
	buf.WriteString("}\n\n")

	buf.WriteString("type numberField interface {\n")
	buf.WriteString("\tsqlingo.NumberField\n")
	buf.WriteString("}\n\n")

	buf.WriteString("type stringField interface {\n")
	buf.WriteString("\tsqlingo.StringField\n")
	buf.WriteString("}\n\n")

	buf.WriteString("type booleanField interface {\n")
	buf.WriteString("\tsqlingo.BooleanField\n")
	buf.WriteString("}\n\n")

	if len(options.tableNames) == 0 {
		options.tableNames, err = schemaFetcher.GetTableNames()
		if err != nil {
			return err
		}
	}
	generateGetTable(buf, options)
	err = WriteToFile(buf, fmt.Sprintf("%s/base.dsl.go", *outputPath), true)
	if err != nil {
		return err
	}

	for _, tableName := range options.tableNames {
		println("Generating", tableName)
		err = generateTable(schemaFetcher, dbName, tableName, options.forceCases)
		if err != nil {
			return err
		}
	}

	return err
}

func generateGetTable(buf *bytes.Buffer, options options) {
	buf.WriteString("func GetTable(name string) sqlingo.Table {\n")
	buf.WriteString("\tswitch name {\n")
	for _, tableName := range options.tableNames {
		buf.WriteString("\tcase " + strconv.Quote(tableName) + ": return " + convertToExportedIdentifier(tableName, options.forceCases) + "\n")
	}
	buf.WriteString("\tdefault: return nil\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	buf.WriteString("func GetTables() []sqlingo.Table {\n")
	buf.WriteString("\treturn []sqlingo.Table{\n")
	for _, tableName := range options.tableNames {
		buf.WriteString("\t" + convertToExportedIdentifier(tableName, options.forceCases) + ",\n")
	}
	buf.WriteString("\t}")
	buf.WriteString("}\n\n")
}

func generateTable(schemaFetcher schemaFetcher, dbName, tableName string, forceCases []string) error {
	fieldDescriptors, err := schemaFetcher.GetFieldDescriptors(tableName)
	if err != nil {
		return err
	}

	className := convertToExportedIdentifier(tableName, forceCases)
	tableStructName := "t" + className
	tableObjectName := "o" + className

	modelClassName := className + "Model"

	var (
		tableLines, modelLines, objectLines, fieldCaseLines, classLines, fields, fieldsSQL, fullFieldsSQL, values bytes.Buffer
	)
	objectLines.WriteString(fmt.Sprintf("\ttable: %s,\n\n", tableObjectName))

	for _, fieldDescriptor := range fieldDescriptors {

		goName := convertToExportedIdentifier(fieldDescriptor.Name, forceCases)
		goType, fieldClass, err := getType(fieldDescriptor)
		if err != nil {
			return err
		}

		privateFieldClass := string(fieldClass[0]+'a'-'A') + fieldClass[1:]

		commentLine := ""
		if fieldDescriptor.Comment != "" {
			commentLine = "\t// " + strings.ReplaceAll(fieldDescriptor.Comment, "\n", " ") + "\n"
		}

		fieldStructName := strings.ToLower(fieldDescriptor.Type) + "_" + className + "_" + goName

		tableLines.WriteString(commentLine)
		tableLines.WriteString(fmt.Sprintf("\t%s %s\n", goName, fieldStructName))

		modelLines.WriteString(commentLine)
		modelLines.WriteString(fmt.Sprintf("\t%s %s\n", goName, goType))

		objectLines.WriteString(commentLine)
		objectLines.WriteString(fmt.Sprintf("\t%s: %s{", goName, fieldStructName))
		objectLines.WriteString(fmt.Sprintf("sqlingo.New%s(%s, %s)},\n", fieldClass, tableObjectName, strconv.Quote(fieldDescriptor.Name)))

		fieldCaseLines.WriteString(fmt.Sprintf("\tcase %s: return t.%s\n", strconv.Quote(fieldDescriptor.Name), goName))

		classLines.WriteString(fmt.Sprintf("type %s struct{ %s }\n", fieldStructName, privateFieldClass))

		fields.WriteString(fmt.Sprintf("t.%s, ", goName))

		if fieldsSQL.Len() != 0 {
			fieldsSQL.WriteString(", ")
		}
		fieldsSQL.WriteString(schemaFetcher.QuoteIdentifier(fieldDescriptor.Name))

		if fullFieldsSQL.Len() != 0 {
			fullFieldsSQL.WriteString(", ")
		}
		fullFieldsSQL.WriteString(fmt.Sprintf("%s.%s", schemaFetcher.QuoteIdentifier(tableName), schemaFetcher.QuoteIdentifier(fieldDescriptor.Name)))

		values.WriteString(fmt.Sprintf("m.%s, ", goName))
	}

	var buf = newBuffWithBaseHeader(dbName)
	buf.WriteString("")
	buf.WriteString(fmt.Sprintf("type %s struct {\n\ttable\n\n", tableStructName))
	buf.WriteString(tableLines.String())
	buf.WriteString("}\n\n")

	buf.WriteString(classLines.String())

	buf.WriteString(fmt.Sprintf("var %s = sqlingo.NewTable(%s)\n", tableObjectName, strconv.Quote(tableName)))
	buf.WriteString(fmt.Sprintf("var %s = %s{\n", className, tableStructName))
	buf.WriteString(objectLines.String())
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (t t%s) GetFields() []sqlingo.Field {\n", className))
	buf.WriteString(fmt.Sprintf("\treturn []sqlingo.Field{%s}\n", fields.String()))
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (t t%s) GetFieldByName(name string) sqlingo.Field {\n", className))
	buf.WriteString("\tswitch name {\n")
	buf.WriteString(fieldCaseLines.String())
	buf.WriteString("\tdefault: return nil\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (t t%s) GetFieldsSQL() string {\n", className))
	buf.WriteString(fmt.Sprintf("\treturn %s\n", strconv.Quote(fieldsSQL.String())))
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (t t%s) GetFullFieldsSQL() string {\n", className))
	buf.WriteString(fmt.Sprintf("\treturn %s\n", strconv.Quote(fullFieldsSQL.String())))
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("type %s struct {\n", modelClassName))
	buf.WriteString(modelLines.String())
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (m %s) GetTable() sqlingo.Table {\n", modelClassName))
	buf.WriteString(fmt.Sprintf("\treturn %s\n", className))
	buf.WriteString("}\n\n")

	buf.WriteString(fmt.Sprintf("func (m %s) GetValues() []interface{} {\n", modelClassName))
	buf.WriteString(fmt.Sprintf("\treturn []interface{}{%s}\n", values.String()))
	buf.WriteString("}\n\n")
	err = WriteToFile(buf, fmt.Sprintf("%s/%s.go", *outputPath, tableName), true)
	return err
}

func WriteToFile(buffer *bytes.Buffer, outputFile string, force bool) error {
	exists, _ := PathExists(outputFile)
	if exists && !force {
		var override string
		fmt.Fprint(os.Stdout, fmt.Sprintf("file(%s) already exists，is overwritten(Y/N)? ", outputFile))
		fmt.Scanln(&override)

		if override != "Y" && override != "y" {
			fmt.Fprintln(os.Stdout, "skip"+outputFile)
			return nil
		}
	}

	f, err := os.OpenFile(outputFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)

	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(buffer.String())

	return nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
