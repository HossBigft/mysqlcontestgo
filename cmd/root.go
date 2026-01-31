package cmd

import (
	"os"

	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mysqlcontestgo",
	Short: "A brief description of your application",
	Run: func(cmd *cobra.Command, args []string) {
		dbhost := os.Getenv("DBHOST")
		dbuser := os.Getenv("DBUSER")
		dbpass := os.Getenv("DBPASS")
		dbport := os.Getenv("DBPORT")

		if dbhost == "" || dbuser == "" || dbpass == "" {
			fmt.Printf("Set DBHOST, DBUSER, DBPASS first")
		}
		fmt.Println("DBHOST: " + dbhost)
		fmt.Println("DBUSER: " + dbuser)
		fmt.Println("DBPASS: " + dbpass[:3])
		fmt.Println("DBPORT: " + dbport)

		data_connect_string := fmt.Sprintf("%s:%s@tcp(%s:%s)/", dbuser, dbpass, dbhost, dbport)
		fmt.Println("Connect string: " + fmt.Sprintf("%s:%s@tcp(%s:%s)/", dbuser, dbpass[:3], dbhost, dbport))

		dbcon, err := sql.Open("mysql", data_connect_string)
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
