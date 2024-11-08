package main

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	println("THE ARGS ARE", strings.Join(os.Args, " "))

	gcsSource := os.Args[1]
	dir := "./sdk"

	if err := copyFilesFromGCS(context.Background(), gcsSource, dir); err != nil {
		fmt.Printf("Failed to copy files: %v\n", err)
	}

	if err := os.Chdir(dir); err != nil {
		fmt.Printf("Failed to change directory: %v\n", err)
		return
	}

	cmd := exec.Command("speakeasy", os.Args[2:]...)

	println("Executing command: ", cmd.String())

	out, err := cmd.CombinedOutput()
	println(string(out))

	if err != nil {
		println("Error executing command: ", err)
		os.Exit(1)
	}
}

// copyFilesFromGCS copies all files from the specified directory (prefix) in the bucket to the local filesystem
func copyFilesFromGCS(ctx context.Context, directory, localPath string) error {
	client, err := storage.NewClient(ctx)
	defer client.Close()
	if err != nil {
		println("Failed to create GCS client", err)
	}

	bucket := client.Bucket("remote-cli-executions")
	query := &storage.Query{Prefix: directory}

	// Create a local directory if it doesn't exist
	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	it := bucket.Objects(ctx, query)

	for {
		// Get the next object
		objAttrs, _ := it.Next()
		if objAttrs == nil {
			break
		}

		// Skip directories (GCS uses empty objects to simulate directories)
		if strings.HasSuffix(objAttrs.Name, "/") {
			continue
		}

		// Download each file
		if err := downloadFile(ctx, bucket, objAttrs.Name, directory, localPath); err != nil {
			return fmt.Errorf("failed to download file %s: %v", objAttrs.Name, err)
		}
	}

	return nil
}

// downloadFile downloads a file from GCS and saves it to the local filesystem
func downloadFile(ctx context.Context, bucket *storage.BucketHandle, objectName, prefix, localPath string) error {
	// Remove the prefix from the object name to create the local file structure
	relativePath := strings.TrimPrefix(objectName, prefix)

	// Join the local path and the relative path to form the full local file path
	localFilePath := filepath.Join(localPath, relativePath)

	// Create any subdirectories required by the file
	if err := os.MkdirAll(filepath.Dir(localFilePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create local directories for %s: %v", localFilePath, err)
	}

	// Create a file on the local filesystem
	f, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %v", localFilePath, err)
	}
	defer f.Close()

	// Get the object from the bucket
	obj := bucket.Object(objectName)
	r, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader for object %s: %v", objectName, err)
	}
	defer r.Close()

	// Copy the contents of the object to the local file
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("failed to copy object %s to local file: %v", objectName, err)
	}

	fmt.Printf("Downloaded file: %s\n", localFilePath)
	return nil
}
