package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// every test points the storage paths and the flavor roots into one temp dir, so
// nothing ever touches /etc/eserved
func setupTestState(t *testing.T) (dir, chrootBase, configBase string) {
	t.Helper()

	dir = t.TempDir()
	chrootBase = filepath.Join(dir, "chroots")
	configBase = filepath.Join(dir, "etc")
	tokensPath = filepath.Join(dir, "tokens.json")
	machinesPath = filepath.Join(dir, "machines.json")
	flavorChrootRoot = func() string { return chrootBase }
	flavorConfigRoot = func() string { return configBase }
	tokens = TokensFile{}
	machines = MachinesFile{}
	if err := LoadTokens(); err != nil {
		t.Fatal(err)
	}
	if err := LoadMachines(); err != nil {
		t.Fatal(err)
	}
	return dir, chrootBase, configBase
}

// the enrollment policy: an unbound token enrolls a new machine only, a bound
// one recovers its own machine, and a token is spent exactly once
func TestEnrollMachinePolicy(t *testing.T) {
	_, chrootBase, configBase := setupTestState(t)

	unbound, err := CreateToken("", "")
	if err != nil {
		t.Fatal(err)
	}
	bound, err := CreateToken("epuller", "build")
	if err != nil {
		t.Fatal(err)
	}

	// a brand new machine enrolls with an unbound token
	if err := EnrollMachine(unbound, "fresh", "build", "fp1"); err != nil {
		t.Fatalf("new machine with an unbound token: %v", err)
	}
	// and that token is spent
	if err := EnrollMachine(unbound, "fresh2", "build", "fp2"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("reusing a spent token: %v", err)
	}

	// an unbound token can't take over a registered machine
	takeover, err := CreateToken("", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnrollMachine(takeover, "fresh", "build", "fp3"); !errors.Is(err, ErrMachineTaken) {
		t.Errorf("takeover of a registered cn: %v", err)
	}
	if !MachineCertValid("fresh", "fp1") {
		t.Error("the refused takeover changed the fingerprint")
	}

	// a token bound to the cn recovers that machine
	if err := EnrollMachine(bound, "epuller", "build", "fp4"); err != nil {
		t.Fatalf("recovery with a cn-bound token: %v", err)
	}
	if !MachineCertValid("epuller", "fp4") {
		t.Error("the recovery didn't take")
	}

	// a cn-bound token can't enroll a different cn
	other, err := CreateToken("epuller", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnrollMachine(other, "someone-else", "build", "fp5"); !errors.Is(err, ErrTokenCN) {
		t.Errorf("cn-bound token enrolling another cn: %v", err)
	}

	// a flavor-bound token can't enroll into a different flavor
	flavored, err := CreateToken("", "build")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnrollMachine(flavored, "flavored", "gnome", "fp6"); !errors.Is(err, ErrTokenFlavor) {
		t.Errorf("flavor-bound token enrolling another flavor: %v", err)
	}
	if err := EnrollMachine(flavored, "flavored", "build", "fp7"); err != nil {
		t.Fatalf("flavor-bound token enrolling its own flavor: %v", err)
	}

	// revocation is sticky, a re-enrollment doesn't clear it
	if err := RevokeMachine("epuller"); err != nil {
		t.Fatal(err)
	}
	recover2, err := CreateToken("epuller", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnrollMachine(recover2, "epuller", "build", "fp8"); err != nil {
		t.Fatalf("re-enrolling a revoked machine: %v", err)
	}
	if MachineCertValid("epuller", "fp8") {
		t.Error("re-enrollment cleared the revocation")
	}

	// bindings are validated at creation
	if _, err := CreateToken("../evil", ""); !errors.Is(err, ErrInvalidTokenBinding) {
		t.Errorf("a path-traversing cn was accepted: %v", err)
	}
	if _, err := CreateToken("", "../evil"); !errors.Is(err, ErrInvalidTokenBinding) {
		t.Errorf("a path-traversing flavor was accepted: %v", err)
	}

	// joining a flavor that already exists needs a token bound to it
	joiner, err := CreateToken("", "")
	if err != nil {
		t.Fatal(err)
	}
	err = EnrollMachine(joiner, "joiner", "build", "fp9")
	if !errors.Is(err, ErrFlavorExists) {
		t.Errorf("an unbound token joined an existing flavor: %v", err)
	}
	if !strings.Contains(err.Error(), "eservectl token create -flavor build") {
		t.Errorf("the refusal doesn't name the fix: %v", err)
	}
	if MachineExists("joiner") {
		t.Error("the refused join registered the machine")
	}
	// the refusal didn't spend the token, and a brand new flavor is still fair game
	if err := EnrollMachine(joiner, "joiner", "freshflavor", "fp9"); err != nil {
		t.Fatalf("an unbound token starting a new flavor: %v", err)
	}
	// a flavor-bound token may join the flavor it is bound to
	joiner2, err := CreateToken("", "build")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnrollMachine(joiner2, "joiner2", "build", "fp10"); err != nil {
		t.Fatalf("a flavor-bound token joining its flavor: %v", err)
	}

	// the rule itself: a provisioned chroot, a machine on the flavor, or a config dir
	if !flavorExistsIn(chrootBase, configBase, "build") {
		t.Error("a flavor with machines on it doesn't exist")
	}
	if flavorExistsIn(chrootBase, configBase, "nope") {
		t.Error("a flavor nobody ever touched exists")
	}
	if err := os.MkdirAll(filepath.Join(configBase, "flavors", "cfgonly"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !flavorExistsIn(chrootBase, configBase, "cfgonly") {
		t.Error("a flavor with a config dir doesn't exist")
	}
	if err := os.MkdirAll(filepath.Join(chrootBase, "provonly", "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chrootBase, "provonly", "etc", "eserved-stage3.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !flavorExistsIn(chrootBase, configBase, "provonly") {
		t.Error("a provisioned flavor doesn't exist")
	}
	if flavorExistsIn(chrootBase, configBase, "../evil") {
		t.Error("an invalid flavor exists")
	}
}

// the flavor-switch policy: the token must be unspent and, if bound, match the
// machine and the requested flavor, and it is spent exactly once
func TestSpendFlavorSwitchTokenPolicy(t *testing.T) {
	_, _, configBase := setupTestState(t)

	// a cn-bound token can't switch another machine
	cnBound, err := CreateToken("advS2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SpendFlavorSwitchToken(cnBound, "advS3", "advt"); !errors.Is(err, ErrTokenCN) {
		t.Errorf("cn-bound token switching another machine: %v", err)
	}
	// and the refusal didn't spend it
	if _, err := SpendFlavorSwitchToken(cnBound, "advS2", "advt"); err != nil {
		t.Fatalf("cn-bound token switching its own machine: %v", err)
	}

	// a flavor-bound token can't switch to another flavor
	flavorBound, err := CreateToken("", "gnome")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SpendFlavorSwitchToken(flavorBound, "advS", "advt"); !errors.Is(err, ErrTokenFlavor) {
		t.Errorf("flavor-bound token switching another flavor: %v", err)
	}
	if _, err := SpendFlavorSwitchToken(flavorBound, "advS", "gnome"); err != nil {
		t.Fatalf("flavor-bound token switching its own flavor: %v", err)
	}

	// a spent token can't be spent again
	if _, err := SpendFlavorSwitchToken(flavorBound, "advS", "gnome"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("double spend: %v", err)
	}

	// an unknown token is refused
	if _, err := SpendFlavorSwitchToken("nope", "advS", "gnome"); !errors.Is(err, ErrTokenUnknown) {
		t.Errorf("unknown token: %v", err)
	}

	// an unbound token switches any machine to any flavor, exactly once
	unbound, err := CreateToken("", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SpendFlavorSwitchToken(unbound, "advS", "advt"); err != nil {
		t.Fatalf("unbound token switching: %v", err)
	}
	if _, err := SpendFlavorSwitchToken(unbound, "advS3", "advt"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("unbound token spent twice: %v", err)
	}

	// switching to a flavor that already exists needs a token bound to it
	if err := os.MkdirAll(filepath.Join(configBase, "flavors", "existing"), 0o755); err != nil {
		t.Fatal(err)
	}
	unbound2, err := CreateToken("", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = SpendFlavorSwitchToken(unbound2, "advS", "existing")
	if !errors.Is(err, ErrFlavorExists) {
		t.Errorf("an unbound token switched to an existing flavor: %v", err)
	}
	if !strings.Contains(err.Error(), "eservectl token create -flavor existing") {
		t.Errorf("the refusal doesn't name the fix: %v", err)
	}
	// the refusal didn't spend it
	if _, err := SpendFlavorSwitchToken(unbound2, "advS", "advt"); err != nil {
		t.Fatalf("spending after a refused switch: %v", err)
	}
	// a flavor-bound token may switch to its flavor
	bound2, err := CreateToken("", "existing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SpendFlavorSwitchToken(bound2, "advS", "existing"); err != nil {
		t.Fatalf("a flavor-bound token switching to its flavor: %v", err)
	}
}

// a token is only refunded while it is still exactly as the spend left it, and a
// provision queued before a switch must not run afterwards and revert the flavor
func TestFlavorSwitchRefundAndStaleJob(t *testing.T) {
	setupTestState(t)

	enroll, err := CreateToken("", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnrollMachine(enroll, "advS", "gnome", "fp"); err != nil {
		t.Fatal(err)
	}

	// the job never started, so the token comes back and can be spent again
	token, err := CreateToken("advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	spent, err := SpendFlavorSwitchToken(token, "advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	if err := spent.Refund(); err != nil {
		t.Fatalf("refunding a token whose job never started: %v", err)
	}
	// and it is still bound to its machine, not left unbound for anyone
	if _, err := SpendFlavorSwitchToken(token, "someone-else", "advt"); !errors.Is(err, ErrTokenCN) {
		t.Errorf("a refunded cn-bound token lost its binding: %v", err)
	}
	if _, err := SpendFlavorSwitchToken(token, "advS", "advt"); err != nil {
		t.Fatalf("spending a refunded token: %v", err)
	}

	// a refund can't revive a token another request spent
	other, err := CreateToken("advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	otherSpent, err := SpendFlavorSwitchToken(other, "advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	if err := RefundFlavorSwitchToken(other, otherSpent.SpentAt.Add(time.Second), otherSpent.PreviousCN); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("refunding with a stale stamp: %v", err)
	}
	if _, err := SpendFlavorSwitchToken(other, "advS", "advt"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("a refused refund un-spent the token: %v", err)
	}

	// a switch that committed keeps its token spent, a second refund is a no-op
	committed, err := CreateToken("advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	committedSpent, err := SpendFlavorSwitchToken(committed, "advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	committedSpent.Commit()
	if err := committedSpent.Refund(); err != nil {
		t.Fatalf("refunding a committed switch: %v", err)
	}
	if _, err := SpendFlavorSwitchToken(committed, "advS", "advt"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("a committed switch was refunded: %v", err)
	}

	// a provision queued while the machine was on gnome must not run after a
	// switch to advt and revert it
	if err := ProvisionMachine("advS", "amd64", "x86_64-pc-linux-gnu", "default/linux/amd64/23.0", "advt", "gnome"); err != nil {
		t.Fatalf("switching to advt: %v", err)
	}
	if flavor, _ := MachineFlavor("advS"); flavor != "advt" {
		t.Fatalf("the switch didn't take, flavor is %v", flavor)
	}
	if err := ProvisionMachine("advS", "amd64", "x86_64-pc-linux-gnu", "default/linux/amd64/23.0", "gnome", "gnome"); err == nil {
		t.Error("a stale job reverted the machine to gnome")
	}
	if flavor, _ := MachineFlavor("advS"); flavor != "advt" {
		t.Errorf("the refused stale job changed the flavor to %v", flavor)
	}
	// a job that still finds its flavor there goes through
	if err := ProvisionMachine("advS", "amd64", "x86_64-pc-linux-gnu", "default/linux/amd64/23.0", "advt", "advt"); err != nil {
		t.Errorf("a current job was refused: %v", err)
	}
}
