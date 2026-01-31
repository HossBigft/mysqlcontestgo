package cmd

import (
	"os"

	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

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

const configFilename = "dbcontest.json"

func loadConfig(path string) (*DBConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return &DBConfig{}, nil
	}
	defer file.Close()

	parsedConfig := &DBConfig{}
	if err := json.NewDecoder(file).Decode(parsedConfig); err != nil {
		return nil, err
	}
	return parsedConfig, nil
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
	Short: "App to test sql connection. On first run asks connection data and saves it.",
	Run: func(cmd *cobra.Command, args []string) {

		cfg, err := loadConfig(configFilename)
		if err != nil {
			fmt.Println("Error loading config:", err)
		} else {
			fmt.Println("Loaded config:", configFilename)
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
				if portInput != "" {
					if p, err := strconv.Atoi(portInput); err == nil && p > 0 && p <= 65535 {
						cfg.Port = p
					} else {
						fmt.Println("Invalid port, using default 3306")
						cfg.Port = 3306
					}
				}
			}

			if err := saveConfig(configFilename, cfg); err != nil {
				fmt.Println("Error saving config:", err)
			} else {
				fmt.Println("Config saved to", configFilename)
			}
		}

		maskedPass := strings.Repeat("*", len(cfg.Pass))
		fmt.Println("DBHOST: " + cfg.Server)
		fmt.Println("DBUSER: " + cfg.User)
		fmt.Println("DBPASS: " + maskedPass)
		fmt.Printf("DBPORT: %v\n", cfg.Port)

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%v)/", cfg.User, cfg.Pass, cfg.Server, cfg.Port)
		fmt.Println("Database DSN: " + fmt.Sprintf("%s:%s@tcp(%s:%v)/", cfg.User, maskedPass, cfg.Server, cfg.Port))

		address := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			fmt.Printf("Cannot reach %s: %v\n", address, err)
		} else {
			fmt.Printf("Host reachable: %s\n", address)
			conn.Close()
		}

		dbcon, err := sql.Open("mysql", dsn)
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
	rootCmd.Flags().StringP("server", "s", "", "Database server IP/Domain")
	rootCmd.Flags().StringP("user", "u", "", "Database user")
	rootCmd.Flags().IntP("port", "p", 0, "Database port")
}
