package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	// "github.com/redis/go-redis/v9"
	"github.com/gomodule/redigo/redis"
)

func main() {
	logger := log.Default()

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
		mux.HandleFunc("/call-b", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// query mysql
			var version string
			if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			logger.Print("Query OK \n")

			// query redis
			_, err := rdb.Do("PING") // this works even if we don't pass the context, wow!
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			serverbURL := os.Getenv("SERVER_B_DATA_URL")
			client := http.DefaultClient
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverbURL, nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()
			fmt.Fprintf(w, "{\"response_status_from_b\": \"%d\", \"mysql_version\": \"%s\"}", resp.StatusCode, version)
		})

		http.ListenAndServe(":8080", mux)
	} else {
		// For alibaba
		r := gin.Default()
		r.GET("/call-b", func(c *gin.Context) {
			ctx := c.Request.Context()

			// query mysql
			var version string
			if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			logger.Print("Query OK \n")

			// query redis
			_, err := rdb.Do("PING") // this works even if we don't pass the context, wow!
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			serverbURL := os.Getenv("SERVER_B_DATA_URL")
			resp, err := http.Get(serverbURL)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer resp.Body.Close()

			c.JSON(http.StatusOK, gin.H{
				"response_status_from_b": resp.StatusCode,
				"mysql_version":          version,
			})
		})

		// Start server
		r.Run(":8080")
	}
}
