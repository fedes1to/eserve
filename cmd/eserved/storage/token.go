package storage

import (
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

type TokenEntry struct {
	CN        string    `json:"cn"`
	Flavor    string    `json:"flavor"`
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

// enrollment refusals, the identity handler maps them to status codes
var (
	ErrTokenUnknown        = errors.New("unknown token")
	ErrTokenUsed           = errors.New("token already used")
	ErrTokenCN             = errors.New("token not bound to this cn")
	ErrTokenFlavor         = errors.New("token not bound to this flavor")
	ErrFlavorExists        = errors.New("flavor already exists")
	ErrInvalidTokenBinding = errors.New("invalid token binding")
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

// cn and flavor are optional bindings: a bound token only enrolls that machine
// (the recovery path) and/or that flavor, an unbound one enrolls anything new
func CreateToken(cn, flavor string) (string, error) {
	if cn != "" && !validCN(cn) {
		return "", fmt.Errorf("%w: invalid cn %q", ErrInvalidTokenBinding, cn)
	}
	if flavor != "" && !chroot.ValidFlavor(flavor) {
		return "", fmt.Errorf("%w: invalid flavor %q", ErrInvalidTokenBinding, flavor)
	}

	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	token := rand.Text()

	if _, tokenExists := tokens.Entries[token]; tokenExists {
		return "", fmt.Errorf("Token already exists... You just stumbled on something almost impossible, or something is really fucked with your PC, Bye!")
	}
	tokens.Entries[token] = TokenEntry{CN: cn, Flavor: flavor, CreatedAt: time.Now()}

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

	// a used token is spent, whatever happened to its machine since
	return tokenToCheck.UsedAt.UTC().IsZero()
}

// the cn ends up in file names and logs, so keep it boring: no dots at the
// edges, no control characters, no unicode
var cnPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9_-])?$`)

func validCN(cn string) bool {
	return len(cn) > 0 && len(cn) <= 64 && cnPattern.MatchString(cn)
}

func ValidCN(token string, cn string) bool {
	tokensMutex.RLock()
	defer tokensMutex.RUnlock()

	entry, exists := tokens.Entries[token]
	if !exists {
		return false
	}
	if !validCN(cn) {
		return false
	}

	if entry.CN != "" && cn != entry.CN {
		return false
	}
	return true
}

func DeleteToken(token string) error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	if _, exists := tokens.Entries[token]; !exists {
		return fmt.Errorf("can't delete non-existent token %s: %w", token, ErrNotFound)
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
			Flavor:    entry.Flavor,
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
