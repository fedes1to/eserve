package api

import (
	"log"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/storage"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/sysinfo"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

func postProvisioning(flavor, stage, token string) error {
	cpuSubarch, err := sysinfo.GetCpuSubarch()
	if err != nil {
		return err
	}

	gccMachine, err := sysinfo.GetGccMachine()
	if err != nil {
		return err
	}

	profile, err := sysinfo.GetPortageProfile()
	if err != nil {
		return err
	}

	log.Printf("Found profile %v with subarch %v\n", profile, cpuSubarch)

	payload := protocol.ProvisionRequest{
		GccMachine: gccMachine,
		Subarch:    cpuSubarch,
		Profile:    profile,
		Stagefile:  stage,
		Flavor:     flavor,
	}
	var provisionResponse protocol.ProvisionResponse
	err = sendMtlsRequestWithToken(urls.ProvisionSuburl, payload, token, &provisionResponse, http.StatusAccepted)
	if err != nil {
		return err
	}

	if err = getStreamJob(provisionResponse.JobID); err != nil {
		return err
	}

	// no -flavor means the server uses whatever it already tracks
	if flavor == "" {
		flavor = provisionResponse.Flavor
	}

	if err = storage.WriteBinhostConfig(flavor, provisionResponse.BinhostURL); err != nil {
		return err
	}

	// a fresh provision may have reset the flavor's config; not fatal, 'epull sync' fixes it
	if err := runSync(false); err != nil {
		log.Printf("portage config may need syncing: %v", err)
	}

	return nil
}

func HandleProvision(flavor, stage, token string) (error, int) {
	log.Printf("Provisioning with flavor %v", flavor)

	return cli.MustRegister([]cli.InitStep{
		{Name: "provisioning", Function: func() error { return postProvisioning(flavor, stage, token) }},
	})
}
