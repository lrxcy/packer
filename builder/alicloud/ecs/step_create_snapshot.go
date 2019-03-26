package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type stepCreateAlicloudSnapshot struct {
	snapshot                 *ecs.Snapshot
	WaitSnapshotReadyTimeout int
}

func (s *stepCreateAlicloudSnapshot) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	client := state.Get("client").(*ecs.Client)
	ui := state.Get("ui").(packer.Ui)

	instance := state.Get("instance").(ecs.Instance)
	describeDisksReq := ecs.CreateDescribeDisksRequest()

	describeDisksReq.RegionId = config.AlicloudRegion
	describeDisksReq.InstanceId = instance.InstanceId
	describeDisksReq.DiskType = DiskType
	disks, err := client.DescribeDisks(describeDisksReq)

	if err != nil {
		return halt(state, err, "Error describe disks")
	}
	disk := disks.Disks.Disk
	if len(disk) == 0 {
		return halt(state, err, "Unable to find system disk of instance")
	}

	// Create the alicloud snapshot
	ui.Say(fmt.Sprintf("Creating snapshot from system disk: %s", disk[0].DiskId))

	createSnapshotReq := ecs.CreateCreateSnapshotRequest()

	createSnapshotReq.DiskId = disk[0].DiskId
	snapshot, err := client.CreateSnapshot(createSnapshotReq)
	if err != nil {
		return halt(state, err, "Error creating snapshot")
	}

	if err := WaitForSnapShotReady(config.AlicloudRegion, snapshot.SnapshotId, s.WaitSnapshotReadyTimeout); err != nil {
		return halt(state, err, "Timeout waiting for snapshot to be created")
	}

	describeSnapshotsReq := ecs.CreateDescribeSnapshotsRequest()

	describeSnapshotsReq.RegionId = config.AlicloudRegion
	describeSnapshotsReq.SnapshotIds = snapshot.SnapshotId
	snapshots, err := client.DescribeSnapshots(describeSnapshotsReq)
	if err != nil {
		return halt(state, err, "Error querying created snapshot")
	}
	snaps := snapshots.Snapshots.Snapshot
	if len(snaps) == 0 {
		return halt(state, err, "Unable to find created snapshot")
	}
	s.snapshot = &snaps[0]
	state.Put("alicloudsnapshot", snapshot.SnapshotId)

	return multistep.ActionContinue
}

func (s *stepCreateAlicloudSnapshot) Cleanup(state multistep.StateBag) {
	if s.snapshot == nil {
		return
	}
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if !cancelled && !halted {
		return
	}

	client := state.Get("client").(*ecs.Client)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Deleting the snapshot because of cancellation or error...")

	deleteSnapshotReq := ecs.CreateDeleteSnapshotRequest()

	deleteSnapshotReq.SnapshotId = s.snapshot.SnapshotId
	if _, err := client.DeleteSnapshot(deleteSnapshotReq); err != nil {
		ui.Error(fmt.Sprintf("Error deleting snapshot, it may still be around: %s", err))
		return
	}
}

func WaitForSnapShotReady(regionId string, snapshotId string, timeout int) error {
	var b Builder
	b.config.AlicloudRegion = regionId
	if err := b.config.Config(); err != nil {
		return err
	}
	client, err := b.config.Client()
	if err != nil {
		return err
	}

	if timeout <= 0 {
		timeout = 60
	}
	for {
		describeSnapshotsReq := ecs.CreateDescribeSnapshotsRequest()

		describeSnapshotsReq.RegionId = regionId
		describeSnapshotsReq.SnapshotIds = snapshotId
		resp, err := client.DescribeSnapshots(describeSnapshotsReq)
		if err != nil {
			return err
		}
		snapshot := resp.Snapshots.Snapshot
		if snapshot != nil || len(snapshot) == 0 {
			return fmt.Errorf("Not found snapshot")
		}
		if snapshot[0].Progress == "100%" {
			break
		}
		timeout = timeout - 5
		if timeout <= 0 {
			return fmt.Errorf("Timeout")
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}
