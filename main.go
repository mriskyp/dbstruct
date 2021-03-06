package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	conv "github.com/mriskyp/dbstruct/convert"
	"github.com/mriskyp/dbstruct/model"
)

const (
	autoGeneratedStructData = `type %s struct {` + "%s" + "\n" + `}`
	tempStructData          = "\t" + `%s %s ` + "`" + `json:"%s"` + ` db:"%s"` + "`"

	mysqlType    = "mysql"
	postgresType = "postgres"

	indexMapGenerate = "generate-dbstruct"
)

func main() {

	cfg, err := generateFromYml()
	if err != nil {
		fmt.Printf("error generate schema from yml. err: %+v", err)
		panic(err)
	}

	fmt.Printf("\n data cfg %+v \n", cfg)
	err = initializeDB(cfg)
	if err != nil {
		fmt.Printf("error generate struct. err: %+v", err)
	} else {
		fmt.Println("success generate struct")
	}
}

func generateFromYml() (*model.Config, error) {

	data, err := ioutil.ReadFile("generate.yml")
	if err != nil {
		return nil, err
	}

	var mapConfig map[string]model.Config

	err = yaml.Unmarshal(data, &mapConfig)
	if err != nil {
		return nil, err
	}

	configData := mapConfig[indexMapGenerate]

	// return data config
	return &configData, nil
}

func initializeDB(cfg *model.Config) error {
	// initialize db
	var db *sql.DB
	var err error

	if cfg == nil {
		err = errors.New("no valid config found")
		return err
	}

	if cfg.JsonFormat == "" {
		err = errors.New("no valid config json format found")
		return err
	}

	isOpenDB := false
	if cfg.DBType == postgresType {
		psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+"password=%s dbname=%s sslmode=disable",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
		dbPsql, err := sql.Open(postgresType, psqlInfo)
		if err != nil {
			return err
		}
		db = dbPsql
		isOpenDB = true
	}

	if cfg.DBType == mysqlType {
		dbMysql, err := sql.Open(mysqlType, cfg.DBUser+":"+cfg.DBPassword+"@tcp("+cfg.DBHost+":"+cfg.DBPort+")/"+cfg.DBName)
		if err != nil {
			return err
		}
		isOpenDB = true
		db = dbMysql
	}
	if !isOpenDB {
		return errors.New("error proceed struct due db is not open")
	}

	defer db.Close()

	err = db.Ping()
	if err != nil {
		return err
	}

	fmt.Println("Successfully connected!")

	start := time.Now()

	if cfg.TableName == "" {
		err = errors.New("invalid table name yml. table name required")
		return err
	}

	rows, err := db.Query(`SELECT column_name , ordinal_position ,is_nullable ,data_type, column_type 
	FROM INFORMATION_SCHEMA.COLUMNS 
	WHERE TABLE_NAME like ? ;
	`, cfg.TableName)

	if err != nil {
		// handle this error better than this
		panic(err)
	}

	fmt.Printf("\n get query rows :%+v \n", rows)

	defer rows.Close()
	var bulkAppendData string

	fmt.Println("\n---------------check type data---------------\n")

	for rows.Next() {
		var columnName, columnType, dataType, isNullable, typeData string
		var ordinalPosition int
		var isNull bool
		err = rows.Scan(&columnName, &ordinalPosition, &isNullable, &dataType, &columnType)
		if err != nil {
			// handle this error
			return err
		}
		if isNullable == "YES" {
			isNull = true
			typeData = "null.%s"
		} else {
			typeData = dataType
		}

		if dataType == "enum" {
			if isNull {
				typeData = fmt.Sprintf(typeData, "String")
			} else {
				typeData = "string"
			}
		}

		if dataType == "date" {
			if isNull {
				typeData = fmt.Sprintf(typeData, "String")
			} else {
				typeData = "string"
			}
		}

		if dataType == "datetime" || dataType == "time" {
			if isNull {
				typeData = fmt.Sprintf(typeData, "Time")
			} else {
				typeData = "*time.Time"
			}
		}

		if strings.Contains(dataType, "int") {
			if isNull {
				typeData = fmt.Sprintf(typeData, "Int")
			} else {
				typeData = "int64"
			}
		}

		if strings.Contains(dataType, "decimal") {
			if isNull {
				typeData = fmt.Sprintf(typeData, "Float")
			} else {
				typeData = "float64"
			}
		}

		if strings.Contains(dataType, "boolean") {
			if isNull {
				typeData = fmt.Sprintf(typeData, "Bool")
			} else {
				typeData = "bool"
			}
		}

		if strings.Contains(dataType, "char") || strings.Contains(dataType, "text") {
			if isNull {
				typeData = fmt.Sprintf(typeData, "String")
			} else {
				typeData = "string"
			}
		}

		/**
		tempdata is store temporary for splitted column name
		appendString is a builder to append formatting tempData
		parserNameType is a string which will show as json name
		parserJSONType is a string which will show as json type
		*/
		var tempData, appendString, parserNameType, parserJSONType string

		splittedColumnName := strings.Split(columnName, "_")
		if len(splittedColumnName) > 0 {
			for i := 0; i < len(splittedColumnName); i++ {
				tempData = conv.UpperInitial(splittedColumnName[i])
				appendString = fmt.Sprintf("%s%s", appendString, tempData)
			}
		} else {
			appendString = conv.UpperInitial(dataType)
		}

		if cfg.JsonFormat == "underscore" {
			parserJSONType = columnName
		} else if cfg.JsonFormat == "camelcase" {
			parserJSONType = conv.LowerInitial(appendString)
		} else {
			err = errors.New("json format not valid")
			return err
		}
		parserNameType = appendString
		// parserNameType = appendString
		// parserJSONType = conv.LowerInitial(parserNameType)

		fmt.Println(columnName, ordinalPosition, isNullable, dataType, columnType)

		appendData := fmt.Sprintf(tempStructData, parserNameType, typeData, parserJSONType, columnName)
		bulkAppendData = fmt.Sprintf("%s\n%s", bulkAppendData, appendData)
	}

	fmt.Println("\n---------------auto generated struct---------------\n")

	if bulkAppendData != "" {
		structName := cfg.StructName
		totalStruct := fmt.Sprintf(autoGeneratedStructData, structName, bulkAppendData)
		fmt.Println(totalStruct)
	}

	elapsed := time.Since(start)
	fmt.Printf("\nquery processing time %+v\n", elapsed)

	return nil
}
