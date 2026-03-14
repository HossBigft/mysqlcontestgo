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
	"golang.org/x/term"
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
	fmt.Fprintf(os.Stderr, "%s: ", fieldName)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func PasswordPrompt(prompt string) (string, error) {
	const (
		keyEnterCR   = '\r'
		keyEnterLF   = '\n'
		keyBackspace = 127
		keyDel       = '\b'
	)
	fmt.Print(prompt)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var password []byte
	buf := make([]byte, 1)

	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}

		switch buf[0] {
		case keyEnterCR, keyEnterLF:
			fmt.Print("\r\n")
			return string(password), nil

		case keyBackspace, keyDel:
			if len(password) > 0 {
				password = password[:len(password)-1]
				fmt.Print("\b \b")
			}

		default:
			password = append(password, buf[0])
			fmt.Print("*")
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "mysqlcontestgo",
	Short: "App to test sql connection. On first run asks connection data and saves it.",
	Run: func(cmd *cobra.Command, args []string) {

		reconfigureFlag, _ := cmd.Flags().GetBool("reconfigure")
		cfg := &DBConfig{}
		var configUpdated bool = false
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
		passwordFlag, _ := cmd.Flags().GetBool("password")

		if passwordFlag {
			cfg.Pass, _ = PasswordPrompt("Database Pass:")
			configUpdated = true
		}

		if !cfg.IsComplete() {

			cfg.Server = promptIfEmpty("Database Host", cfg.Server)
			cfg.User = promptIfEmpty("Database User", cfg.User)
			if len(cfg.Pass) == 0 {
				cfg.Pass, _ = PasswordPrompt("Database Pass:")
			}

			if cfg.Port == 0 {
				fmt.Fprintf(os.Stderr, "Database Port (default 3306): \n")
				var portInput string
				fmt.Scanln(&portInput)
				if portInput != "" {
					if p, err := strconv.Atoi(portInput); err == nil && p > 0 && p <= 65535 {
						cfg.Port = p
					}
				} else {
					fmt.Fprintf(os.Stderr, "Using default port 3306\n")
					cfg.Port = 3306
				}
			}
			configUpdated = true
		}
		if configUpdated {
			err := saveConfig(configFilename, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config:%q\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Config saved to %q\n", configFilename)
			}
		}
		maskedPass := strings.Repeat("*", len(cfg.Pass))
		fmt.Fprintf(os.Stderr, "DatabaseHOST: %s\n", cfg.Server)
		fmt.Fprintf(os.Stderr, "DatabaseUSER: %s\n", cfg.User)
		fmt.Fprintf(os.Stderr, "DatabasePASS: %s\n", maskedPass)
		fmt.Fprintf(os.Stderr, "DatabasePORT: %v\n", cfg.Port)

		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%v)/",
			cfg.User,
			cfg.Pass,
			cfg.Server,
			cfg.Port,
		)
		DSN := fmt.Sprintf("%s:%s@tcp(%s:%v)", cfg.User, maskedPass, cfg.Server, cfg.Port)
		fmt.Fprintf(os.Stderr, "\nDatabase DSN: %s\n", DSN)

		var resolvedIP string

		if net.ParseIP(cfg.Server) == nil {
			ips, err := net.LookupHost(cfg.Server)
			if err != nil {
				var dnsErr *net.DNSError
				if errors.As(err, &dnsErr) {
					fmt.Fprintf(os.Stderr, "DNS resolution failed for domain %s: %v\n", cfg.Server, dnsErr)
				} else {
					fmt.Fprintf(os.Stderr, "Unknown error resolving domain %s: %v\n", cfg.Server, err)
				}
				return
			}

			fmt.Fprintf(os.Stderr, "Domain %s resolved to IP(s): %v\n", cfg.Server, ips)
			resolvedIP = ips[0]
		} else {
			resolvedIP = cfg.Server
		}

		tcpTimeoutSeconds := 5 * time.Second
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", resolvedIP, cfg.Port), tcpTimeoutSeconds)
		if err != nil {
			fmt.Fprintf(os.Stderr, "TCP connection failed to %s:%d in %f seconds: %v\n", resolvedIP, cfg.Port, tcpTimeoutSeconds.Seconds(), err)
			return
		}

		fmt.Fprintf(os.Stderr, "Host reachable: %s:%d\n", resolvedIP, cfg.Port)
		conn.Close()

		localAddr := "unknown"
		if conn != nil {
			localAddr = conn.LocalAddr().String()
		}
		fmt.Fprintf(os.Stderr, "Client source address: %s\n", localAddr)

		dbcon, err := sql.Open("mysql", dsn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open connection: %v\n", err)
			return
		}
		defer dbcon.Close()

		err = dbcon.Ping()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot connect:\n")

			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) {
				fmt.Fprintf(os.Stderr, "MySQL error code: %d\n", mysqlErr.Number)
				fmt.Fprintf(os.Stderr, "MySQL SQLState: %s\n", mysqlErr.SQLState)
				fmt.Fprintf(os.Stderr, "MySQL message: %s\n", mysqlErr.Message)
			}

			return
		}

		fmt.Fprintf(os.Stderr, "Connected successfully!")

		fmt.Fprintf(os.Stderr, "\nRunning SELECT @@port...")
		var port int
		err = dbcon.QueryRow("SELECT @@port").Scan(&port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query: @@port failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "MySQL is running on port: %d\n", port)
		}

		grants, err := dbcon.Query("SHOW GRANTS")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query: SHOW GRANTS failed: %v\n", err)
		}
		defer grants.Close()

		fmt.Fprintf(os.Stderr, "\nGrants for %s:\n", cfg.User)
		for grants.Next() {
			var grant string
			if err := grants.Scan(&grant); err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning grant: %v\n", err)
				continue
			}
			fmt.Println(grant)
		}

		fmt.Fprintf(os.Stderr, "\nPrinting available databases:")
		databases, err := dbcon.Query("SHOW DATABASES")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v", err)
		}
		defer databases.Close()

		for databases.Next() {
			var name string
			databases.Scan(&name)
			fmt.Fprintf(os.Stderr, " -%s", name)
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
	rootCmd.Flags().IntP("port", "P", 0, "Database port")
	rootCmd.Flags().BoolP("reconfigure", "r", false, "Prompt for config values")
	rootCmd.Flags().BoolP("password", "p", false, "Prompt for password")
}
