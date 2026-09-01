package storage

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

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
	tokensPath  string = filepath.Join(serverConfig.ServerConfigPath, "tokens.json")
)

func LoadTokens() error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()
	if err := loadTokensLocked(); err != nil {
		return err
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func saveTokensLocked() error {
	if err := config.SafeSaveJsonFile(tokensPath, tokens); err != nil {
		return err
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func loadTokensLocked() error {
	if err := config.LoadJsonFile(tokensPath, &tokens); err != nil {
		return err
	}

	if tokens.Entries == nil {
		tokens.Entries = make(map[string]TokenEntry)
	}

	return nil
}

func CreateToken() (string, error) {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	token := rand.Text()

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

	// a used token is one-shot: if its CN is already enrolled, its spent
	if tokenToCheck.CN != "" || !tokenToCheck.UsedAt.UTC().IsZero() {
		machinesMutex.RLock()
		_, machineExists := machines.Entries[tokenToCheck.CN]
		machinesMutex.RUnlock()
		if machineExists {
			return false
		}
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
	// the cn ends up in file names and logs: same boring rules as flavors
	if cn == "." || cn == ".." || strings.ContainsAny(cn, "/\\ ") {
		return false
	}

	if entry.CN != "" && cn != entry.CN {
		return false
	}
	return true
}

func UseToken(token string, cn string) error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	if entry, ok := tokens.Entries[token]; ok && (entry.CN != "" || !entry.UsedAt.UTC().IsZero()) {
		return fmt.Errorf("token already used")
	}

	if !isTokenAvailableLocked(token) {
		return fmt.Errorf("Token became invalid at usage")
	}

	tokenToUse := tokens.Entries[token]
	if tokenToUse.CN != "" && tokenToUse.CN != cn {
		return fmt.Errorf("token not valid for machine")
	}
	tokenToUse.CN = cn
	tokenToUse.UsedAt = time.Now()
	tokens.Entries[token] = tokenToUse

	return saveTokensLocked()
}

func DeleteToken(token string) error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	if _, exists := tokens.Entries[token]; !exists {
		return fmt.Errorf("can't delete non-existent token %s", token)
	}

	delete(tokens.Entries, token)
	return saveTokensLocked()
}

// returns the tokens, oldest first
func ListTokens() []protocol.TokenInfo {
	tokensMutex.RLock()
	defer tokensMutex.RUnlock()

	list := make([]protocol.TokenInfo, 0, len(tokens.Entries))
	for token, entry := range tokens.Entries {
		list = append(list, protocol.TokenInfo{
			Token:     token,
			CN:        entry.CN,
			CreatedAt: entry.CreatedAt,
			UsedAt:    entry.UsedAt,
		})
	}
	slices.SortFunc(list, func(a, b protocol.TokenInfo) int {
		switch {
		case a.CreatedAt.Before(b.CreatedAt):
			return -1
		case a.CreatedAt.After(b.CreatedAt):
			return 1
		default:
			return strings.Compare(a.Token, b.Token)
		}
	})
	return list
}
