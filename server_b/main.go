package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo"

	// "github.com/redis/go-redis/v9"
	"github.com/gomodule/redigo/redis"
)

func main() {
	e := echo.New()

	mysqlDSN := os.Getenv("MYSQL_DSN")
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	redisAddr := os.Getenv("REDIS_ADDR")
	// NOTE: go-redis didn't work on my computer... something wrong?
	// rdb := redis.NewClient(&redis.Options{
	// 	Addr: redisAddr,
	// })
	// defer rdb.Close()
	rdb, err := redis.Dial("tcp", redisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()

	serverMode := os.Getenv("SERVER_MODE")
	if serverMode == "net/http" {
		// For otel
		mux := http.NewServeMux()
		mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// query mysql
			var version string
			if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// query redis
			_, err := rdb.Do("PING") // this works even if we don't pass the context, wow!
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			fmt.Fprintln(w, "{\"message\": \"Hello from server B!\", \"mysql_version\": \"%s\"}", version)
		})

		http.ListenAndServe(":8081", mux)
	} else {
		// Endpoint to return data
		e.GET("/data", func(c echo.Context) error {
			ctx := c.Request().Context()

			// query mysql
			var version string
			if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			// query redis
			_, err := rdb.Do("PING") // this works even if we don't pass the context, wow!
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			return c.JSON(http.StatusOK, map[string]string{
				"message":       "Hello from server B!",
				"mysql_version": version,
			})
		})

		// Start server
		e.Logger.Fatal(e.Start(":8081"))
	}
}
