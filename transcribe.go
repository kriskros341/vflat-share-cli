package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const geminiURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

const prompt = "Transcribe the note on this page. Return only the text, without any " +
	"additional commentary. Try to preserve the formatting where possible - " +
	"spacing and lines separating the different sections of the note." +
	"Please keep to the original language of the note (they may be mixed)"

// Request/response shapes for the Gemini generateContent REST call.
type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type geminiRequest struct {
	Contents []struct {
		Parts []part `json:"parts"`
	} `json:"contents"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func transcribeImage(apiKey, model, imagePath string) (string, error) {
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(imageBytes)

	var reqBody geminiRequest
	reqBody.Contents = make([]struct {
		Parts []part `json:"parts"`
	}, 1)
	reqBody.Contents[0].Parts = []part{
		{Text: prompt},
		{InlineData: &inlineData{MimeType: "image/jpeg", Data: encoded}},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf(geminiURL, model) + "?" + url.Values{"key": {apiKey}}.Encode()
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini API error %d: %s", resp.StatusCode, string(body))
	}

	var result geminiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}

func transcribeAll(apiKey, model string, files []fileInfo, outDir string) {
	fmt.Printf("Starting transcription of %d files...\n", len(files))
	var wg sync.WaitGroup
	for _, fi := range files {
		wg.Add(1)
		go func(fi fileInfo) {
			defer wg.Done()
			imagePath := filepath.Join(outDir, fi.Filename)
			text, err := transcribeImage(apiKey, model, imagePath)
			if err != nil {
				fmt.Printf("Failed to transcribe %s: %v\n", fi.Filename, err)
				return
			}
			stem := strings.TrimSuffix(fi.Filename, filepath.Ext(fi.Filename))
			txtPath := filepath.Join(outDir, stem+".txt")
			if err := os.WriteFile(txtPath, []byte(text), 0o644); err != nil {
				fmt.Printf("Failed to write %s: %v\n", txtPath, err)
				return
			}
			fmt.Printf("Transcription of %s finished!\n", fi.Filename)
		}(fi)
	}
	wg.Wait()
	fmt.Println("Transcription finished!")
}
