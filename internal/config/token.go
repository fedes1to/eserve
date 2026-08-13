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
	tokensMutex sync.RWMutex
	tokensPath  string = ServerConfigPath + "tokens.json"
)

func LoadTokens() error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()
	if loadError := loadTokensLocked(); loadError != nil {
		return loadError
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func saveTokensLocked() error {
	if saveError := SafeSaveJsonFile(tokensPath, tokens); saveError != nil {
		return saveError
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func loadTokensLocked() error {
	if loadError := LoadJsonFile(tokensPath, &tokens); loadError != nil {
		return loadError
	}

	if tokens.Entries == nil {
		tokens.Entries = make(map[string]TokenEntry)
	}

	return nil
}

func CreateToken() (string, error) {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := fmt.Sprintf("%x", tokenBytes)

	if _, tokenExists := tokens.Entries[token]; tokenExists {
		return "", fmt.Errorf("Token already exists... You just stumbled on something almost impossible, or something is really fucked with your PC, Bye!")
	}
	entry := tokens.Entries[token]
	entry.CreatedAt = time.Now()
	tokens.Entries[token] = entry

	return token, saveTokensLocked()
}

func IsTokenAvailable(token string) bool {
	tokensMutex.RLock()
	defer tokensMutex.RUnlock()

	return isTokenAvailableLocked(token)
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func isTokenAvailableLocked(token string) bool {
	tokenToCheck, tokenExists := tokens.Entries[token]
	if !tokenExists {
		return false
	}

	if tokenToCheck.CN != "" || !tokenToCheck.UsedAt.UTC().IsZero() {
		if _, machineExists := machines.Entries[token]; machineExists {
			return false
		}
		log.Println("Token is used but not in machines, continuing...")
	}
	return true
}

func ValidCN(token string, cn string) bool {
	tokensMutex.RLock()
	defer tokensMutex.RUnlock()

	entry, exists := tokens.Entries[token]
	if !exists || cn == "" {
		return false
	}

	// for new cns
	if entry.CN != "" && cn != entry.CN {
		return false
	}
	return true
}

func UseToken(token string, cn string) error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	if !isTokenAvailableLocked(token) {
		return fmt.Errorf("Token became invalid at usage")
	}

	tokenToUse := tokens.Entries[token]
	tokenToUse.CN = cn
	tokenToUse.UsedAt = time.Now()
	tokens.Entries[token] = tokenToUse

	return saveTokensLocked()
}
