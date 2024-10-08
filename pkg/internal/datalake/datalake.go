// pkg/internal/datalake/datalake.go
package datalake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
)

// SeaweedFSClient represents a client for interacting with SeaweedFS
type SeaweedFSClient struct {
	MasterURL string
	VolumeURL string
}

// NewSeaweedFSClient creates a new SeaweedFSClient
func NewSeaweedFSClient() *SeaweedFSClient {
	return &SeaweedFSClient{
		MasterURL: os.Getenv("SEAWEEDFS_MASTER"),
		VolumeURL: os.Getenv("SEAWEEDFS_VOLUME"),
	}
}

// UploadFile uploads a file to SeaweedFS
func (c *SeaweedFSClient) UploadFile(filename string, data []byte) (string, error) {
	uploadURL, err := c.getUploadURL()
	if err != nil {
		return "", err
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	writer.Close()

	request, err := http.NewRequest("POST", uploadURL, &requestBody)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to upload file, status: %s, body: %s", resp.Status, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	fid, ok := result["fid"].(string)
	if !ok {
		return "", fmt.Errorf("fid not found in response or not a string: %+v", result)
	}

	return fid, nil
}

// DownloadFile downloads a file from SeaweedFS
func (c *SeaweedFSClient) DownloadFile(fileID string) ([]byte, error) {
	downloadURL := fmt.Sprintf("http://%s/%s", c.VolumeURL, fileID)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file, status: %s", resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}

// DeleteFile deletes a file from SeaweedFS
func (c *SeaweedFSClient) DeleteFile(fileID string) error {
	deleteURL := fmt.Sprintf("http://%s/%s", c.VolumeURL, fileID)
	request, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete file, status: %s", resp.Status)
	}

	return nil
}

// getUploadURL gets an upload URL from the SeaweedFS master
func (c *SeaweedFSClient) getUploadURL() (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/dir/assign", c.MasterURL))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get upload URL, status: %s", resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s/%s", result["publicUrl"], result["fid"]), nil
}
