package main

import (
	_ "embed"
	"errors"
	"io"

	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-redis/redis/v8"
)

//go:embed index.html
var index string

func main() {
	config := readConfig()

	http.HandleFunc("/config", configHandler(config))
	http.HandleFunc("/ping", pingHandler(config))
	http.HandleFunc("/s3", s3Handler(config))

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, index)
	})

	log.Default().Println("listening...")

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// configHandler handles the /config endpoint
func configHandler(config map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Default().Println("/config")

		err := writeJSONResponse(w, config)
		if err != nil {
			log.Fatal(err)
		}
	}
}

type PingResult struct {
	Value string `json:"value,omitempty"`
	Err   error  `json:"error,omitempty"`
}

// pingHandler handles the /ping endpoint
func pingHandler(config map[string]string) http.HandlerFunc {

	rdb := redis.NewClient(&redis.Options{
		Addr:     config["DB_HOST"],
		Password: config["redis-password"],
	})

	return func(w http.ResponseWriter, req *http.Request) {
		log.Default().Println("/ping")

		value, pingError := rdb.Ping(context.Background()).Result()

		pingResult := PingResult{
			Value: value,
			Err:   pingError,
		}

		err := writeJSONResponse(w, pingResult)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func s3Handler(config map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Default().Println("/s3")

		sess := session.Must(session.NewSession())
		svc := s3.New(sess, &aws.Config{
			Region: aws.String("eu-central-1"),
		})
		out, err := svc.ListBuckets(nil)

		log.Default().Printf("out: %+v - err: %+v\n", out, err)
	}
}

func readConfig() map[string]string {
	config := make(map[string]string)

	err := filepath.WalkDir("/configurations", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		fileInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}

		if fileInfo.Mode().IsRegular() {
			fileContent, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			log.Default().Println("Setting up config: ", dirEntry.Name())

			config[dirEntry.Name()] = string(fileContent)
		}

		return nil
	})

	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Fatal(err)
		}
	}

	return config
}

func writeJSONResponse(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	return encoder.Encode(v)
}
