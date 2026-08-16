package api

import (
	"fmt"
	"io"
	"strings"
)

func GetStages() (string, error) {
	response, err := mtlsClient.Get("/api/v1/stages")
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
	stages := strings.Split(stagesString, "\n")
	for i, stage := range stages {
		fmt.Printf("[%v] %v\n", i, stage)
	}
	fmt.Print("Select your stage: ")
	var stageIndex int
	fmt.Scanln("%v", &stageIndex)

	return stages[stageIndex], nil
}
