package config

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"
)

// these are pretty much only for logging the tokens and registering
type TokenEntry struct {
	CN        string    `json:"cn"`
	CreatedAt time.Time `json:"created"`
	UsedAt    time.Time `json:"used"`
}

type TokensFile struct {
	Entries map[string]TokenEntry `json:"tokens"`
}

var (
	tokens      TokensFile
	tokensMutex sync.Mutex
)

func CreateToken() error {
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := fmt.Sprintf("%x", tokenBytes)

	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	if _, tokenExists := tokens.Entries[token]; tokenExists {
		return fmt.Errorf("Token already exists... You just stumbled on something almost impossible, or something is really fucked with your PC, Bye!")
	}
	entry := tokens.Entries[token]
	entry.CreatedAt = time.Now()
	tokens.Entries[token] = entry

	return nil
}

func IsTokenAvailable(token string) bool {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	tokenToCheck, tokenExists := tokens.Entries[token]
	if tokenToCheck.CN != "" || !tokenToCheck.UsedAt.UTC().IsZero() {
		if _, machineExists := machines.Entries[token]; machineExists {
			return false
		}
		log.Println("Token is used but not in machines, continuing...")
	}
	return tokenExists
}

func UseToken(token string, cn string) error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	tokenToUse, tokenExists := tokens.Entries[token]
	if !tokenExists {
		return fmt.Errorf("Token doesn't exist")
	}

	if tokenToUse.CN != "" || !tokenToUse.UsedAt.UTC().IsZero() {
		if _, machineOk := machines.Entries[token]; machineOk {
			return fmt.Errorf("Tried to use a used token")
		}
		log.Println("Token is used but not in machines, continuing...")
	}

	tokenToUse.CN = cn
	tokenToUse.UsedAt = time.Now()
	tokens.Entries[token] = tokenToUse

	return nil

}
