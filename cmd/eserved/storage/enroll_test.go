package storage

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// the enrollment policy: an unbound token enrolls a new machine only, a bound
// one recovers its own machine, and a token is spent exactly once
func TestEnrollMachinePolicy(t *testing.T) {
	dir := t.TempDir()
	tokensPath = filepath.Join(dir, "tokens.json")
	machinesPath = filepath.Join(dir, "machines.json")
	tokens = TokensFile{}
	machines = MachinesFile{}
	if err := LoadTokens(); err != nil {
		t.Fatal(err)
	}
	if err := LoadMachines(); err != nil {
		t.Fatal(err)
	}

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
}

// the flavor-switch policy: the token must be unspent and, if bound, match the
// machine and the requested flavor, and it is spent exactly once
func TestSpendFlavorSwitchTokenPolicy(t *testing.T) {
	dir := t.TempDir()
	tokensPath = filepath.Join(dir, "tokens.json")
	machinesPath = filepath.Join(dir, "machines.json")
	tokens = TokensFile{}
	machines = MachinesFile{}
	if err := LoadTokens(); err != nil {
		t.Fatal(err)
	}
	if err := LoadMachines(); err != nil {
		t.Fatal(err)
	}

	// a cn-bound token can't switch another machine
	cnBound, err := CreateToken("advS2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := SpendFlavorSwitchToken(cnBound, "advS3", "advt"); !errors.Is(err, ErrTokenCN) {
		t.Errorf("cn-bound token switching another machine: %v", err)
	}
	// and the refusal didn't spend it
	if _, _, err := SpendFlavorSwitchToken(cnBound, "advS2", "advt"); err != nil {
		t.Fatalf("cn-bound token switching its own machine: %v", err)
	}

	// a flavor-bound token can't switch to another flavor
	flavorBound, err := CreateToken("", "gnome")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := SpendFlavorSwitchToken(flavorBound, "advS", "advt"); !errors.Is(err, ErrTokenFlavor) {
		t.Errorf("flavor-bound token switching another flavor: %v", err)
	}
	if _, _, err := SpendFlavorSwitchToken(flavorBound, "advS", "gnome"); err != nil {
		t.Fatalf("flavor-bound token switching its own flavor: %v", err)
	}

	// a spent token can't be spent again
	if _, _, err := SpendFlavorSwitchToken(flavorBound, "advS", "gnome"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("double spend: %v", err)
	}

	// an unknown token is refused
	if _, _, err := SpendFlavorSwitchToken("nope", "advS", "gnome"); !errors.Is(err, ErrTokenUnknown) {
		t.Errorf("unknown token: %v", err)
	}

	// an unbound token switches any machine to any flavor, exactly once
	unbound, err := CreateToken("", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := SpendFlavorSwitchToken(unbound, "advS", "advt"); err != nil {
		t.Fatalf("unbound token switching: %v", err)
	}
	if _, _, err := SpendFlavorSwitchToken(unbound, "advS3", "advt"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("unbound token spent twice: %v", err)
	}
}

// a token is only refunded while it is still exactly as the spend left it, and a
// provision queued before a switch must not run afterwards and revert the flavor
func TestFlavorSwitchRefundAndStaleJob(t *testing.T) {
	dir := t.TempDir()
	tokensPath = filepath.Join(dir, "tokens.json")
	machinesPath = filepath.Join(dir, "machines.json")
	tokens = TokensFile{}
	machines = MachinesFile{}
	if err := LoadTokens(); err != nil {
		t.Fatal(err)
	}
	if err := LoadMachines(); err != nil {
		t.Fatal(err)
	}

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
	spentAt, previousCN, err := SpendFlavorSwitchToken(token, "advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	if err := RefundFlavorSwitchToken(token, spentAt, previousCN); err != nil {
		t.Fatalf("refunding a token whose job never started: %v", err)
	}
	// and it is still bound to its machine, not left unbound for anyone
	if _, _, err := SpendFlavorSwitchToken(token, "someone-else", "advt"); !errors.Is(err, ErrTokenCN) {
		t.Errorf("a refunded cn-bound token lost its binding: %v", err)
	}
	if _, _, err := SpendFlavorSwitchToken(token, "advS", "advt"); err != nil {
		t.Fatalf("spending a refunded token: %v", err)
	}

	// a refund can't revive a token another request spent
	other, err := CreateToken("advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	otherAt, otherCN, err := SpendFlavorSwitchToken(other, "advS", "advt")
	if err != nil {
		t.Fatal(err)
	}
	if err := RefundFlavorSwitchToken(other, otherAt.Add(time.Second), otherCN); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("refunding with a stale stamp: %v", err)
	}
	if _, _, err := SpendFlavorSwitchToken(other, "advS", "advt"); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("a refused refund un-spent the token: %v", err)
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
