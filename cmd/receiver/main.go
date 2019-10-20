package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const numConcurrentUploads = 6

const md5Postfix = ".md5"

var spacesToken = os.Getenv("SPACES_TOKEN")
var spacesSecret = os.Getenv("SPACES_SECRET")

var activeRequests = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "photo_uploader_active_requests",
	Help: "Number of active connections",
})
var totalRequests = promauto.NewCounter(prometheus.CounterOpts{
	Name: "photo_uploader_total_requests",
	Help: "Number of total requests",
})
var hashSumMiss = promauto.NewCounter(prometheus.CounterOpts{
	Name: "photo_uploader_hash_sum_miss",
	Help: "Number of failed requests due to incorrect hash sums",
})

func uploadPhoto(w http.ResponseWriter, r *http.Request) {
	activeRequests.Inc()
	defer activeRequests.Dec()
	totalRequests.Inc()

	file, header, err := r.FormFile("file")
	if err != nil {
		logrus.Panic(err)
	}

	fileName := header.Filename
	hashSum := r.FormValue("hashsum")
	logrus.WithField("hashsum", hashSum).Info("New file")

	fileHash := md5.New()
	if _, err := io.Copy(fileHash, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logrus.Panic(err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logrus.Panic(err)
	}

	endpoint := "https://fra1.digitaloceanspaces.com"
	bucket := "brews"
	sess := session.New(&aws.Config{
		Endpoint:    &endpoint,
		Region:      aws.String("us-east-1"),
		Credentials: credentials.NewStaticCredentials(spacesToken, spacesSecret, ""),
	})
	svc := s3.New(sess)

	hashBytes := fileHash.Sum(nil)
	hashBytesHexString := hex.EncodeToString(hashBytes)
	hashBytesBase64String := base64.StdEncoding.EncodeToString(hashBytes)

	if hashBytesHexString != hashSum {
		http.Error(w, "Hashes are not identical!", http.StatusBadRequest)
		logrus.WithField("server_hash", string(hashBytes)).WithField("client_hash", hashSum).Warn("Invalid request")
		return
	}
	resp, err := svc.PutObject(&s3.PutObjectInput{
		Bucket:     &bucket,
		Key:        &fileName,
		Body:       file,
		ContentMD5: aws.String(hashBytesBase64String),
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logrus.Panic(err)
	}

	if s3Hash := strings.Trim(*resp.ETag, "\""); s3Hash != hashBytesHexString {
		http.Error(w, "MD5 sums does not match after upload", http.StatusInternalServerError)
		logrus.WithField("server_hash", hashBytesBase64String).WithField("s3_hash", s3Hash).Panic("MD5 does not match")
	}
	hashedHashBytes := md5.Sum([]byte(hashBytesHexString))
	hashedHashBytesBase64String := base64.StdEncoding.EncodeToString(hashedHashBytes[:16])
	hashedHashBytesHexString := hex.EncodeToString(hashedHashBytes[:16])
	resp, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:     &bucket,
		Key:        aws.String(fileName + md5Postfix),
		Body:       bytes.NewReader([]byte(hashBytesHexString)),
		ContentMD5: aws.String(hashedHashBytesBase64String),
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logrus.Panic(err)
	}

	if s3Hash := strings.Trim(*resp.ETag, "\""); s3Hash != hashedHashBytesHexString {
		http.Error(w, "MD5 sums does not match after upload", http.StatusInternalServerError)
		logrus.WithField("server_hash", hashBytesBase64String).WithField("s3_hash", s3Hash).Panic("MD5 does not match")
	}
}

func main() {
	http.HandleFunc("/photos/upload", uploadPhoto)
	go http.ListenAndServe(":2112", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}