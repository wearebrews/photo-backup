package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const numConcurrentUploads = 1

const md5Postfix = ".md5"

var spacesToken = os.Getenv("SPACES_TOKEN")
var spacesSecret = os.Getenv("SPACES_SECRET")
var bucket = "brews"

var activeRequests = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "photo_uploader",
	Name:      "active_requests",
	Help:      "Number of active connections",
})
var totalRequests = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "photo_uploader",
	Name:      "total_requests",
	Help:      "Number of total requests",
})
var requestsDenied = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "photo_uploader",
	Name:      "denied_requests",
	Help:      "Number of requests denied",
})

func hexToBase64(in string) (string, error) {
	hexBytes, err := hex.DecodeString(in)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(hexBytes), nil
}

func uploadPhoto(sem chan struct{}, uploader *s3manager.Uploader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//Limit number of concurrent requests
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			requestsDenied.Inc()
			http.Error(w, "Not available, try again", http.StatusTooManyRequests)
			return
		}

		activeRequests.Inc()
		defer activeRequests.Dec()
		totalRequests.Inc()

		reader, err := r.MultipartReader()
		if err != nil {
			logrus.Panic(err)
		}

		part, err := reader.NextPart()
		if err != nil {
			logrus.Panic(err)
		}
		if part.FormName() != "hash_sum" {
			http.Error(w, "Unexpected part", http.StatusBadRequest)
			return
		}
		var hashSum string
		if bytes, err := ioutil.ReadAll(part); err == nil {
			hashSum = string(bytes)
		} else {
			logrus.Panic(err)
		}

		logrus.WithField("hashsum", hashSum).WithField("size", fileSize).Info("New file")

		part, err = reader.NextPart()
		if err != nil {
			logrus.Panic(err)
		}

		if part.FormName() != "file" {
			http.Error(w, "Unexpected part", http.StatusBadRequest)
			return
		}

		hash := md5.New()
		tr := io.TeeReader(part, hash)

		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: &bucket,
			Key:    aws.String("pictures/" + part.FileName()),
			Body:   tr,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logrus.Panic(err)
		}

		hashBytes := hash.Sum(nil)
		hashHexString := hex.EncodeToString(hashBytes[:16])
		if hashHexString != hashSum {
			http.Error(w, "Hash does not match", http.StatusInternalServerError)
			logrus.WithField("client_hash", hashSum).WithField("server_hash", hashHexString).Panic("Hash does not match")
		}
	}
}

func main() {
	endpoint := "https://fra1.digitaloceanspaces.com"
	sess := session.New(&aws.Config{
		Endpoint:    &endpoint,
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(spacesToken, spacesSecret, ""),
	})
	sem := make(chan struct{})
	uploader := s3manager.NewUploader(sess)
	promMux := http.NewServeMux()
	promMux.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/photos/upload", uploadPhoto(sem, uploader))
	go http.ListenAndServe(":9102", promMux)
	http.ListenAndServe(":8080", nil)
}
