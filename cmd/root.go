package cmd

import (
	"os"

	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
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

		reconfigureFlag, _ := cmd.Flags().GetBool("reconfigure")
		cfg := &DBConfig{}

		if !reconfigureFlag {
			loadedConfig, err := loadConfig(configFilename)
			if err == nil {
				cfg = loadedConfig
			}
		}
		if server, _ := cmd.Flags().GetString("server"); server != "" {
			cfg.Server = server
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
					}
				} else {
					fmt.Println("Invalid port, using default 3306")
					cfg.Port = 3306
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

		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%v)/",
			cfg.User,
			cfg.Pass,
			cfg.Server,
			cfg.Port,
		)
		fmt.Println("\nDatabase DSN: " + fmt.Sprintf("%s:%s@tcp(%s:%v)/", cfg.User, maskedPass, cfg.Server, cfg.Port))

		var resolvedIP string

		if net.ParseIP(cfg.Server) == nil {
			ips, err := net.LookupHost(cfg.Server)
			if err != nil {
				var dnsErr *net.DNSError
				if errors.As(err, &dnsErr) {
					fmt.Printf("DNS resolution failed for domain %s: %v\n", cfg.Server, dnsErr)
				} else {
					fmt.Printf("Unknown error resolving domain %s: %v\n", cfg.Server, err)
				}
				return
			}

			fmt.Printf("Domain %s resolved to IP(s): %v\n", cfg.Server, ips)
			resolvedIP = ips[0]
		} else {
			resolvedIP = cfg.Server
		}

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", resolvedIP, cfg.Port), 5*time.Second)
		if err != nil {
			fmt.Printf("TCP connection failed to %s:%d: %v\n", resolvedIP, cfg.Port, err)
			return
		}

		fmt.Printf("Host reachable: %s:%d\n", resolvedIP, cfg.Port)
		conn.Close()

		localAddr := "unknown"
		if conn != nil {
			localAddr = conn.LocalAddr().String()
		}
		fmt.Println("Client source address:", localAddr)

		dbcon, err := sql.Open("mysql", dsn)
		if err != nil {
			fmt.Printf("Failed to open connection: %v\n", err)
			return
		}
		defer dbcon.Close()

		err = dbcon.Ping()
		if err != nil {
			fmt.Printf("Cannot connect:\n")

			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) {
				fmt.Printf("MySQL error code: %d\n", mysqlErr.Number)
				fmt.Printf("MySQL SQLState: %s\n", mysqlErr.SQLState)
				fmt.Printf("MySQL message: %s\n", mysqlErr.Message)
			}

			return
		}

		fmt.Println("Connected successfully!")

		fmt.Println("\nRunning SELECT @@port...")
		var port int
		err = dbcon.QueryRow("SELECT @@port").Scan(&port)
		if err != nil {
			fmt.Printf("Query: @@port failed: %v\n", err)
		} else {
			fmt.Println("MySQL is running on port:", port)
		}

		grants, err := dbcon.Query("SHOW GRANTS")
		if err != nil {
			fmt.Printf("Query: SHOW GRANTS failed: %v\n", err)
		}
		defer grants.Close()

		fmt.Printf("\nGrants for %s:\n", cfg.User)
		for grants.Next() {
			var grant string
			if err := grants.Scan(&grant); err != nil {
				fmt.Printf("Error scanning grant: %v\n", err)
				continue
			}
			fmt.Println(grant)
		}

		fmt.Println("\nPrinting available databases:")
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
	rootCmd.Flags().BoolP("reconfigure", "r", false, "Prompt for config values again")
}
