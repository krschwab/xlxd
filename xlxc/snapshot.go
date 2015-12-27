package main

import (
	"fmt"

	"github.com/krschwab/xlxd"
	"github.com/krschwab/xlxd/i18n"
	"github.com/krschwab/xlxd/shared"
	"github.com/krschwab/xlxd/shared/gnuflag"
)

type snapshotCmd struct {
	stateful bool
}

func (c *snapshotCmd) showByDefault() bool {
	return true
}

func (c *snapshotCmd) usage() string {
	return i18n.G(
		`Create a read-only snapshot of a container.

lxc snapshot [remote:]<source> <snapshot name> [--stateful]`)
}

func (c *snapshotCmd) flags() {
	gnuflag.BoolVar(&c.stateful, "stateful", false, i18n.G("Whether or not to snapshot the container's running state"))
}

func (c *snapshotCmd) run(config *lxd.Config, args []string) error {
	if len(args) < 1 {
		return errArgs
	}

	var snapname string
	if len(args) < 2 {
		snapname = ""
	} else {
		snapname = args[1]
	}

	remote, name := config.ParseRemoteAndContainer(args[0])
	d, err := lxd.NewClient(config, remote)
	if err != nil {
		return err
	}

	// we don't allow '/' in snapshot names
	if shared.IsSnapshot(snapname) {
		return fmt.Errorf(i18n.G("'/' not allowed in snapshot name"))
	}

	resp, err := d.Snapshot(name, snapname, c.stateful)
	if err != nil {
		return err
	}

	return d.WaitForSuccess(resp.Operation)
}
