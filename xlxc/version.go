package main

import (
	"fmt"

	"github.com/krschwab/xlxd"
	"github.com/krschwab/xlxd/i18n"
	"github.com/krschwab/xlxd/shared"
)

type versionCmd struct{}

func (c *versionCmd) showByDefault() bool {
	return true
}

func (c *versionCmd) usage() string {
	return i18n.G(
		`Prints the version number of LXD.

lxc version`)
}

func (c *versionCmd) flags() {
}

func (c *versionCmd) run(_ *lxd.Config, args []string) error {
	if len(args) > 0 {
		return errArgs
	}
	fmt.Println(shared.Version)
	return nil
}
