package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/flavorlock"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

// a lookup by name found nothing; the admin handlers answer 404 for it
var ErrNotFound = errors.New("not found")

// an unbound token may enroll a new machine, never take over a registered one
var ErrMachineTaken = errors.New("machine already registered, use a token bound to this cn")

type MachineEntry struct {
	Subarch     string         `json:"march"`
	Profile     chroot.Profile `json:"profile"`
	Flavor      string         `json:"flavor"`
	Fingerprint string         `json:"fingerprint"`
	RevokedAt   time.Time      `json:"revoked_at"` // IsZero if not revoked
}

type MachinesFile struct {
	Entries map[string]MachineEntry `json:"machines"`
}

var (
	machines      MachinesFile
	machinesMutex sync.RWMutex
	machinesPath  = filepath.Join(serverConfig.ServerConfigPath, "machines.json")
)

func LoadMachines() error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()
	if err := loadMachinesLocked(); err != nil {
		return err
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func saveMachinesLocked() error {
	if err := config.SafeSaveJsonFile(machinesPath, machines); err != nil {
		return err
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func loadMachinesLocked() error {
	if err := config.LoadJsonFile(machinesPath, &machines); err != nil {
		return err
	}

	if machines.Entries == nil {
		machines.Entries = make(map[string]MachineEntry)
	}

	return nil
}

func RevokeMachine(cn string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	machineEntry, exists := machines.Entries[cn]
	if !exists {
		return fmt.Errorf("Can't revoke non-existant machine %v: %w", cn, ErrNotFound)
	}

	machineEntry.RevokedAt = time.Now()
	machines.Entries[cn] = machineEntry

	return saveMachinesLocked()
}

func DeleteMachine(cn string) error {
	machinesMutex.Lock()
	entry, exists := machines.Entries[cn]
	if !exists {
		machinesMutex.Unlock()
		return fmt.Errorf("can't delete non-existent machine %s: %w", cn, ErrNotFound)
	}
	delete(machines.Entries, cn)
	err := saveMachinesLocked()
	machinesMutex.Unlock()
	if err != nil {
		return err
	}

	if entry.Flavor == "" {
		return nil
	}

	unlock := flavorlock.Lock(entry.Flavor)
	defer unlock()

	// the archive goes either way, or a later flavor apply re-applies the config of
	// a machine that no longer exists
	if err := os.Remove(chroot.SyncArchivePath(entry.Flavor, cn)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if fingerprint, ok := FlavorFingerprintInfo(entry.Flavor); ok && fingerprint.SyncedBy == cn {
		return SetFlavorFingerprint(entry.Flavor, "", "")
	}
	return nil
}

// the flavor the machine had when the request was made: a job queued before a
// concurrent switch must not run afterwards and silently revert the machine
func ProvisionMachine(cn, subarch, gccMachine, profile, flavor, expectedFlavor string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	upsertedEntry, exists := machines.Entries[cn]
	if !exists || upsertedEntry.Fingerprint == "" {
		return fmt.Errorf("machine %v has no certificate — run identity first", cn)
	}
	if upsertedEntry.Flavor != expectedFlavor {
		return fmt.Errorf("machine %v is on flavor %v now, this provision was queued for %v", cn, upsertedEntry.Flavor, expectedFlavor)
	}

	if refusal := chroot.ArchRefusal(flavor, gccMachine); refusal != "" {
		return errors.New(refusal)
	}

	entry := MachineEntry{
		Subarch:     subarch,
		Profile:     chroot.Profile{Full: profile, GccMachine: gccMachine},
		Flavor:      flavor,
		Fingerprint: upsertedEntry.Fingerprint,
		RevokedAt:   upsertedEntry.RevokedAt, // revocation is sticky, a re-provision doesnt clear it
	}
	machines.Entries[cn] = entry
	return saveMachinesLocked()
}

func MachineFlavor(cn string) (string, bool) {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	entry, exists := machines.Entries[cn]
	return entry.Flavor, exists
}

func MachineExists(cn string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	_, exists := machines.Entries[cn]
	return exists
}

// identity enrolls a machine before the provision job runs, so an entry with no
// gcc machine is one whose provision never completed (the handler rejects an
// empty gcc_machine, so a completed provision always has one)
func MachineProvisioned(cn string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	entry, exists := machines.Entries[cn]
	return exists && entry.Profile.GccMachine != ""
}

// consumes the token and registers the machine under one lock, so two identities
// racing for the same cn can't both win
func EnrollMachine(token, cn, flavor, fingerprint string) error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	tokenEntry, exists := tokens.Entries[token]
	if !exists {
		return ErrTokenUnknown
	}
	if !tokenEntry.UsedAt.UTC().IsZero() {
		return ErrTokenUsed
	}
	if tokenEntry.CN != "" && tokenEntry.CN != cn {
		return ErrTokenCN
	}
	if tokenEntry.Flavor != "" && tokenEntry.Flavor != flavor {
		return ErrTokenFlavor
	}
	if _, machineExists := machines.Entries[cn]; machineExists && tokenEntry.CN != cn {
		return ErrMachineTaken
	}
	if err := flavorJoinRefusalLocked(tokenEntry.Flavor, cn, flavor); err != nil {
		return err
	}

	tokenEntry.CN = cn
	tokenEntry.UsedAt = time.Now()
	tokens.Entries[token] = tokenEntry
	if err := saveTokensLocked(); err != nil {
		return err
	}

	// an existing entry keeps its subarch/profile/revocation, only the cert changes
	machineEntry := machines.Entries[cn]
	machineEntry.Fingerprint = fingerprint
	machineEntry.Flavor = flavor
	machines.Entries[cn] = machineEntry
	return saveMachinesLocked()
}

// a spent flavor-switch token: refundable until the switch commits, and only once
type SwitchToken struct {
	Token      string
	SpentAt    time.Time
	PreviousCN string
	done       bool
}

// the switch is written, the token stays spent
func (s *SwitchToken) Commit() {
	if s != nil {
		s.done = true
	}
}

// hands the token back unless the switch committed; a second call is a no-op
func (s *SwitchToken) Refund() error {
	if s == nil || s.done {
		return nil
	}
	s.done = true
	return RefundFlavorSwitchToken(s.Token, s.SpentAt, s.PreviousCN)
}

// the flavor-switch policy: the token must be unspent and, if bound, match the
// machine and the requested flavor; it is consumed in the same lock as the check,
// so one token can only ever authorize one switch. The returned token can be
// refunded until the switch commits
func SpendFlavorSwitchToken(token, cn, flavor string) (*SwitchToken, error) {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	tokenEntry, exists := tokens.Entries[token]
	if !exists {
		return nil, ErrTokenUnknown
	}
	if !tokenEntry.UsedAt.UTC().IsZero() {
		return nil, ErrTokenUsed
	}
	if tokenEntry.CN != "" && tokenEntry.CN != cn {
		return nil, ErrTokenCN
	}
	if tokenEntry.Flavor != "" && tokenEntry.Flavor != flavor {
		return nil, ErrTokenFlavor
	}
	if err := flavorJoinRefusalLocked(tokenEntry.Flavor, cn, flavor); err != nil {
		return nil, err
	}

	previousCN := tokenEntry.CN
	tokenEntry.CN = cn
	tokenEntry.UsedAt = time.Now()
	tokens.Entries[token] = tokenEntry

	if err := saveTokensLocked(); err != nil {
		return nil, err
	}
	return &SwitchToken{Token: token, SpentAt: tokenEntry.UsedAt, PreviousCN: previousCN}, nil
}

// un-spends a token, but only while it is still exactly as the matching spend left
// it, so a refund can never revive a token another request used. Restoring the cn
// the spend replaced keeps a bound token bound
func RefundFlavorSwitchToken(token string, spentAt time.Time, previousCN string) error {
	tokensMutex.Lock()
	defer tokensMutex.Unlock()

	tokenEntry, exists := tokens.Entries[token]
	if !exists {
		return ErrTokenUnknown
	}
	if !tokenEntry.UsedAt.Equal(spentAt) {
		return ErrTokenUsed
	}

	tokenEntry.CN = previousCN
	tokenEntry.UsedAt = time.Time{}
	tokens.Entries[token] = tokenEntry

	return saveTokensLocked()
}

func MachineCertValid(cn, fingerprint string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()
	entry, exists := machines.Entries[cn]
	if !exists {
		return false
	}
	return entry.Fingerprint != "" && entry.Fingerprint == fingerprint && entry.RevokedAt.IsZero()
}

// returns the registered machines, sorted by cn
func ListMachines() []protocol.MachineInfo {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	list := make([]protocol.MachineInfo, 0, len(machines.Entries))
	for cn, entry := range machines.Entries {
		list = append(list, protocol.MachineInfo{
			CN:          cn,
			Subarch:     entry.Subarch,
			Profile:     entry.Profile.Full,
			Flavor:      entry.Flavor,
			Fingerprint: entry.Fingerprint,
			RevokedAt:   entry.RevokedAt,
		})
	}
	slices.SortFunc(list, func(a, b protocol.MachineInfo) int { return strings.Compare(a.CN, b.CN) })
	return list
}
