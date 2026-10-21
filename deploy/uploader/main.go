package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func loadDotEnv(filepaths ...string) {
	for _, fp := range filepaths {
		f, err := os.Open(fp)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				if _, exists := os.LookupEnv(k); !exists || os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}
		_ = f.Close()
	}
}

const (
	// Fixed Cloudflare R2 storage credentials for Release Distribution
	defaultReleaseBucketName     = "centaccess"
	defaultReleaseEndpoint       = "https://38b5c4ba61e33ff3450486cfa645d3ff.r2.cloudflarestorage.com"
	defaultReleasePublicEndpoint = "https://storage.centaccess.com"
	defaultReleaseAccessKeyID    = "59013bc36534654e99bd27ce4f64b596"
	defaultReleaseSecretKey      = "ea13b63374058970c811dddb856ee7bec13bfa78561ac1dd30fc15f5f672684d"
)

func main() {
	loadDotEnv(".env", ".env.local", ".env.example")

	versionFlag := flag.String("version", "latest", "Release version (default: latest)")
	flag.Parse()

	version := *versionFlag
	if version == "" {
		version = "latest"
	}

	// Use fixed Cloudflare R2 endpoints for release deployment so that local dev .env doesn't break release
	bucketName := defaultReleaseBucketName
	endpoint := defaultReleaseEndpoint
	publicEndpoint := defaultReleasePublicEndpoint
	accessKey := defaultReleaseAccessKeyID
	secretKey := defaultReleaseSecretKey
	usePathStyle := true

	// Optional overrides if explicitly provided with RELEASE_* prefix
	if envVal := os.Getenv("RELEASE_S3_BUCKET_NAME"); envVal != "" {
		bucketName = envVal
	}
	if envVal := os.Getenv("RELEASE_S3_ENDPOINT"); envVal != "" {
		endpoint = envVal
	}
	if envVal := os.Getenv("RELEASE_S3_PUBLIC_ENDPOINT"); envVal != "" {
		publicEndpoint = envVal
	}
	if envVal := os.Getenv("RELEASE_S3_ACCESS_KEY_ID"); envVal != "" {
		accessKey = envVal
	}
	if envVal := os.Getenv("RELEASE_S3_SECRET_ACCESS_KEY"); envVal != "" {
		secretKey = envVal
	}

	awsCfg := &aws.Config{
		Region:           aws.String("auto"),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(usePathStyle),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
	}

	sess, err := session.NewSession(awsCfg)
	if err != nil {
		log.Fatalf("❌ Error creating AWS session: %v", err)
	}

	client := s3.New(sess)

	filesToUpload := []struct {
		localPath string
		fileName  string
	}{
		{localPath: "deploy/sea-value-job-linux-amd64", fileName: "sea-value-job-linux-amd64"},
		{localPath: "deploy/sea-value-job-linux-arm64", fileName: "sea-value-job-linux-arm64"},
		{localPath: "deploy/sea-value-job-windows-amd64.exe", fileName: "sea-value-job-windows-amd64.exe"},
		{localPath: "deploy/install.exe", fileName: "install.exe"},
		{localPath: "deploy/install.sh", fileName: "install.sh"},
	}

	log.Printf("🚀 [Sea Value Job S3 Uploader] Deploying Version: %s\n", version)

	for _, item := range filesToUpload {
		s3Key := fmt.Sprintf("releases/custom/sea-value-job/%s/%s", version, item.fileName)
		fmt.Printf("☁️ Uploading %s -> s3://%s/%s ...\n", item.localPath, bucketName, s3Key)

		file, err := os.Open(item.localPath)
		if err != nil {
			log.Fatalf("❌ Failed to open %s: %v", item.localPath, err)
		}

		stat, _ := file.Stat()
		contentType := mime.TypeByExtension(filepath.Ext(item.localPath))
		if filepath.Ext(item.localPath) == ".sh" {
			contentType = "text/x-shellscript"
		} else if contentType == "" {
			contentType = "application/octet-stream"
		}

		_, err = client.PutObject(&s3.PutObjectInput{
			Bucket:        aws.String(bucketName),
			Key:           aws.String(s3Key),
			Body:          file,
			ContentLength: aws.Int64(stat.Size()),
			ContentType:   aws.String(contentType),
			ACL:           aws.String("public-read"),
		})
		file.Close()

		if err != nil {
			log.Fatalf("❌ Upload failed for %s: %v", item.localPath, err)
		}
		fmt.Printf("✅ Uploaded %s (%s) successfully!\n", item.fileName, version)

		// If a specific version was provided (not latest), also copy/upload to latest pointer
		if version != "latest" {
			latestKey := fmt.Sprintf("releases/custom/sea-value-job/latest/%s", item.fileName)
			fmt.Printf("🔄 Updating latest pointer -> s3://%s/%s ...\n", bucketName, latestKey)
			_, err = client.CopyObject(&s3.CopyObjectInput{
				Bucket:     aws.String(bucketName),
				CopySource: aws.String(fmt.Sprintf("%s/%s", bucketName, s3Key)),
				Key:        aws.String(latestKey),
				ACL:        aws.String("public-read"),
			})
			if err != nil {
				log.Printf("⚠️ Warning: Failed to copy to latest pointer: %v", err)
			}
		}
	}

	fmt.Printf("\n🎉 All release files for version [%s] uploaded successfully to S3 / Cloudflare R2!\n", version)
	fmt.Printf("🌐 Public Base URL: %s/releases/custom/sea-value-job/%s\n", publicEndpoint, version)
}
