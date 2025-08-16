package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const (
	githubUsername = "EthicalGT"
	repoName       = "EthicalPay"
	branch         = "main"
)

func getSHAOfFile(filePath string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		githubUsername, repoName, filePath, branch)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+"xxxxxx")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorBody bytes.Buffer
		_, _ = errorBody.ReadFrom(resp.Body)

		fmt.Printf("🔍 GitHub GET Error for file: %s\nStatus Code: %d\nResponse: %s\n",
			filePath, resp.StatusCode, errorBody.String())

		return "", fmt.Errorf("failed to fetch SHA for %s, status code: %d", filePath, resp.StatusCode)
	}

	var data struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode GitHub SHA response: %w", err)
	}
	return data.SHA, nil
}

func pushFileToGitHub(localFilePath, repoFilePath string) error {
	fileContent, err := os.ReadFile(localFilePath)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %w", localFilePath, err)
	}

	sha, err := getSHAOfFile(repoFilePath)
	if err != nil {
		return fmt.Errorf("error fetching SHA for %s: %w", repoFilePath, err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s",
		githubUsername, repoName, repoFilePath)

	payload := map[string]interface{}{
		"message": "Update " + repoFilePath + " from Koyeb",
		"content": base64.StdEncoding.EncodeToString(fileContent),
		"sha":     sha,
		"branch":  branch,
		"committer": map[string]string{
			"name":  "Koyeb App",
			"email": "koyeb@app.com",
		},
	}
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", url, bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "token "+"xxxxxx")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("GitHub push failed for %s, status: %d", repoFilePath, resp.StatusCode)
	}

	return nil
}
