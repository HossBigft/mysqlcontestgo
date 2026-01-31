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
	Server string `json:"server"`
	User   string `json:"user"`
	Pass   string `json:"pass"`
	Port   int    `json:"port"`
}

func (cfg *DBConfig) IsComplete() bool {
	return cfg.Server != "" && cfg.User != "" && cfg.Pass != "" && cfg.Port != 0
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
	Short: "App to test sql connection. On first run asks for data for connection.",
	Run: func(cmd *cobra.Command, args []string) {

		cfg, err := loadConfig(CONFIG_FILENAME)
		if err != nil {
			fmt.Println("Error loading config:", err)
		} else {
			fmt.Println("Loaded config:", CONFIG_FILENAME)
		}

		if host, _ := cmd.Flags().GetString("host"); host != "" {
			cfg.Server = host
		}
		if user, _ := cmd.Flags().GetString("user"); user != "" {
			cfg.User = user
		}
		if port, _ := cmd.Flags().GetInt("port"); port != 0 {
			cfg.Port = port
		}

		if !cfg.IsComplete() {
			cfg.Server = promptIfEmpty("DB Host", cfg.Server)
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

		dbserver := cfg.Server
		dbuser := cfg.User
		dbpass := cfg.Pass
		dbport := cfg.Port
		fmt.Println("DBHOST: " + dbserver)
		fmt.Println("DBUSER: " + dbuser)
		fmt.Println("DBPASS: " + dbpass[:3])
		fmt.Printf("DBPORT: %v\n", dbport)

		dbDataSourceString := fmt.Sprintf("%s:%s@tcp(%s:%v)/", dbuser, dbpass, dbserver, dbport)
		fmt.Println("Database DSN: " + fmt.Sprintf("%s:%s@tcp(%s:%v)/", dbuser, dbpass[:3], dbserver, dbport))

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
	rootCmd.Flags().StringP("server", "s", "", "Database host IP/Domain")
	rootCmd.Flags().StringP("user", "u", "", "Database user")
	rootCmd.Flags().IntP("port", "p", 0, "Database port")
}
