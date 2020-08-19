package model

type Config struct {
	DBHost     string `yaml:"dbHost"`
	DBPort     string `yaml:"dbPort"`
	DBName     string `yaml:"dbName"`
	DBUser     string `yaml:"dbUser"`
	DBPassword string `yaml:"dbPassword"`
	DBType     string `yaml:"dbType"`
	TableName  string `yaml:"tableName"`
}
