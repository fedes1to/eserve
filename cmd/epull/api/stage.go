package api

import (
	"fmt"
	"io"
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
)

func GetStages() (string, error) {
	response, err := mtlsClient.Get(clientConfig.Settings.Server + "/api/v1/stages")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	bodyString := string(bodyBytes)
	if response.StatusCode != 200 {
		return "", fmt.Errorf("couldn't list stagefiles, code %v, body:\n%v", response.StatusCode, bodyString)
	}

	return bodyString, nil
}

func AskStagefile() (string, error) {
	stagesString, err := GetStages()
	if err != nil {
		return "", err
	}
	if stagesString == "" {
		return "", fmt.Errorf("Server has no stages, please download one first")
	}
	stages := strings.Split(stagesString, "\n")
	for i, stage := range stages {
		if stage != "" {
			fmt.Printf("[%v] %v\n", i+1, stage)
		}
	}
	fmt.Print("Select your stage: ")
	var stageIndex int
	if _, err := fmt.Scanln(&stageIndex); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if stageIndex < 1 || stageIndex > len(stages) {
		return "", fmt.Errorf("stage %d out of range", stageIndex)
	}

	return stages[stageIndex-1], nil
}
