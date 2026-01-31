package cmd

import (
	"os"

	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

type DBConfig struct {
	Host string `json:"host"`
	User string `json:"user"`
	Pass string `json:"pass"`
	Port int    `json:"port"`
}

func (cfg *DBConfig) IsComplete() bool {
	return cfg.Host != "" && cfg.User != "" && cfg.Pass != "" && cfg.Port != 0
}

const CONFIG_FILENAME = "dbcontest.json"

func loadConfig(path string) (*DBConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return &DBConfig{}, nil
	}
	defer file.Close()

	parsed_config := &DBConfig{}
	if err := json.NewDecoder(file).Decode(parsed_config); err != nil {
		return nil, err
	}
	return parsed_config, nil
}

func saveConfig(path string, config *DBConfig) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

func promptIfEmpty(fieldName string, current string) string {
	if current != "" {
		return current
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", fieldName)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)

}

var rootCmd = &cobra.Command{
	Use:   "mysqlcontestgo",
	Short: "A brief description of your application",
	Run: func(cmd *cobra.Command, args []string) {

		cfg, err := loadConfig(CONFIG_FILENAME)
		if err != nil {
			fmt.Println("Error loading config:", err)
		} else {
			fmt.Println("Loaded config:", CONFIG_FILENAME)
		}

		if !cfg.IsComplete() {
			cfg.Host = promptIfEmpty("DB Host", cfg.Host)
			cfg.User = promptIfEmpty("DB User", cfg.User)
			cfg.Pass = promptIfEmpty("DB Password", cfg.Pass)
			if cfg.Port == 0 {
				fmt.Printf("DB Port (default 3306): ")
				var portInput string
				fmt.Scanln(&portInput)
				if portInput == "" {
					cfg.Port = 3306
				} else {
					fmt.Sscanf(portInput, "%d", &cfg.Port)
				}
			}

			if err := saveConfig(CONFIG_FILENAME, cfg); err != nil {
				fmt.Println("Error saving config:", err)
			} else {
				fmt.Println("Config saved to", CONFIG_FILENAME)
			}
		}

		dbhost := cfg.Host
		dbuser := cfg.User
		dbpass := cfg.Pass
		dbport := cfg.Port
		fmt.Println("DBHOST: " + dbhost)
		fmt.Println("DBUSER: " + dbuser)
		fmt.Println("DBPASS: " + dbpass[:3])
		fmt.Printf("DBPORT: %v\n", dbport)

		dbDataSourceString := fmt.Sprintf("%s:%s@tcp(%s:%v)/", dbuser, dbpass, dbhost, dbport)
		fmt.Println("Database DSN: " + fmt.Sprintf("%s:%s@tcp(%s:%v)/", dbuser, dbpass[:3], dbhost, dbport))

		dbcon, err := sql.Open("mysql", dbDataSourceString)
		if err != nil {
			fmt.Printf("Failed to open connection: %v\n", err)
			return
		}
		defer dbcon.Close()

		err = dbcon.Ping()
		if err != nil {
			fmt.Printf("Cannot connect: %v\n", err)
			return
		}

		fmt.Println("Connected successfully!")

		databases, err := dbcon.Query("SHOW DATABASES")
		if err != nil {
			fmt.Printf("Query failed: %v", err)
		}
		defer databases.Close()

		for databases.Next() {
			var name string
			databases.Scan(&name)
			fmt.Println(" -", name)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}
