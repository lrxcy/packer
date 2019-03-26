package ecs

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type stepShareAlicloudImage struct {
	AlicloudImageShareAccounts   []string
	AlicloudImageUNShareAccounts []string
	RegionId                     string
}

func (s *stepShareAlicloudImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*ecs.Client)
	ui := state.Get("ui").(packer.Ui)
	alicloudImages := state.Get("alicloudimages").(map[string]string)
	for copiedRegion, copiedImageId := range alicloudImages {
		modifyImageSharePermissionReq := ecs.CreateModifyImageSharePermissionRequest()

		modifyImageSharePermissionReq.RegionId = copiedRegion
		modifyImageSharePermissionReq.ImageId = copiedImageId
		modifyImageSharePermissionReq.AddAccount = &s.AlicloudImageShareAccounts
		modifyImageSharePermissionReq.RemoveAccount = &s.AlicloudImageUNShareAccounts
		if _, err := client.ModifyImageSharePermission(modifyImageSharePermissionReq); err != nil {
			state.Put("error", err)
			ui.Say(fmt.Sprintf("Failed modifying image share permissions: %s", err))
			return multistep.ActionHalt
		}
	}
	return multistep.ActionContinue
}

func (s *stepShareAlicloudImage) Cleanup(state multistep.StateBag) {
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if cancelled || halted {
		ui := state.Get("ui").(packer.Ui)
		client := state.Get("client").(*ecs.Client)
		alicloudImages := state.Get("alicloudimages").(map[string]string)
		ui.Say("Restoring image share permission because cancellations or error...")
		for copiedRegion, copiedImageId := range alicloudImages {
			modifyImageSharePermissionReq := ecs.CreateModifyImageSharePermissionRequest()

			modifyImageSharePermissionReq.RegionId = copiedRegion
			modifyImageSharePermissionReq.ImageId = copiedImageId
			modifyImageSharePermissionReq.AddAccount = &s.AlicloudImageUNShareAccounts
			modifyImageSharePermissionReq.RemoveAccount = &s.AlicloudImageShareAccounts
			if _, err := client.ModifyImageSharePermission(modifyImageSharePermissionReq); err != nil {
				ui.Say(fmt.Sprintf("Restoring image share permission failed: %s", err))
			}
		}
	}
}
