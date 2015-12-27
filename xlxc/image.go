package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"

	"github.com/krschwab/xlxd"
	"github.com/krschwab/xlxd/i18n"
	"github.com/krschwab/xlxd/shared"
	"github.com/krschwab/xlxd/shared/gnuflag"
)

type imageCmd struct{}

func (c *imageCmd) showByDefault() bool {
	return true
}

var imageEditHelp string = i18n.G(
	`### This is a yaml representation of the image properties.
### Any line starting with a '# will be ignored.
###
### Each property is represented by a single line:
### An example would be:
###  description: My custom image`)

func (c *imageCmd) usage() string {
	return i18n.G(
		`Manipulate container images.

lxc image import <tarball> [rootfs tarball|URL] [target] [--public] [--created-at=ISO-8601] [--expires-at=ISO-8601] [--fingerprint=FINGERPRINT] [prop=value]

lxc image copy [remote:]<image> <remote>: [--alias=ALIAS].. [--copy-aliases] [--public]
lxc image delete [remote:]<image>
lxc image export [remote:]<image>
lxc image info [remote:]<image>
lxc image list [remote:] [filter]
lxc image show [remote:]<image>
lxc image edit [remote:]<image>
    Edit image, either by launching external editor or reading STDIN.
    Example: lxc image edit <image> # launch editor
             cat image.yml | lxc image edit <image> # read from image.yml

Lists the images at specified remote, or local images.
Filters are not yet supported.

lxc image alias create <alias> <target>
lxc image alias delete <alias>
lxc image alias list [remote:]

Create, delete, list image aliases. Example:
lxc remote add store2 images.linuxcontainers.org
lxc image alias list store2:`)
}

type aliasList []string

func (f *aliasList) String() string {
	return fmt.Sprint(*f)
}

func (f *aliasList) Set(value string) error {
	if f == nil {
		*f = make(aliasList, 1)
	} else {
		*f = append(*f, value)
	}
	return nil
}

var addAliases aliasList
var publicImage bool = false
var copyAliases bool = false

func (c *imageCmd) flags() {
	gnuflag.BoolVar(&publicImage, "public", false, i18n.G("Make image public"))
	gnuflag.BoolVar(&copyAliases, "copy-aliases", false, i18n.G("Copy aliases from source"))
	gnuflag.Var(&addAliases, "alias", i18n.G("New alias to define at target"))
}

func doImageAlias(config *lxd.Config, args []string) error {
	var remote string
	switch args[1] {
	case "list":
		/* alias list [<remote>:] */
		if len(args) > 2 {
			remote, _ = config.ParseRemoteAndContainer(args[2])
		} else {
			remote, _ = config.ParseRemoteAndContainer("")
		}
		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}

		resp, err := d.ListAliases()
		if err != nil {
			return err
		}

		showAliases(resp)

		return nil
	case "create":
		/* alias create [<remote>:]<alias> <target> */
		if len(args) < 4 {
			return errArgs
		}
		remote, alias := config.ParseRemoteAndContainer(args[2])
		target := args[3]
		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}
		/* TODO - what about description? */
		err = d.PostAlias(alias, alias, target)
		return err
	case "delete":
		/* alias delete [<remote>:]<alias> */
		if len(args) < 3 {
			return errArgs
		}
		remote, alias := config.ParseRemoteAndContainer(args[2])
		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}
		err = d.DeleteAlias(alias)
		return err
	}
	return errArgs
}

func (c *imageCmd) run(config *lxd.Config, args []string) error {
	var remote string

	if len(args) < 1 {
		return errArgs
	}

	switch args[0] {
	case "alias":
		if len(args) < 2 {
			return errArgs
		}
		return doImageAlias(config, args)

	case "copy":
		/* copy [<remote>:]<image> [<rmeote>:]<image> */
		if len(args) != 3 {
			return errArgs
		}
		remote, inName := config.ParseRemoteAndContainer(args[1])
		if inName == "" {
			return errArgs
		}
		destRemote, outName := config.ParseRemoteAndContainer(args[2])
		if outName != "" {
			return errArgs
		}
		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}
		dest, err := lxd.NewClient(config, destRemote)
		if err != nil {
			return err
		}
		image := dereferenceAlias(d, inName)
		return d.CopyImage(image, dest, copyAliases, addAliases, publicImage)

	case "delete":
		/* delete [<remote>:]<image> */
		if len(args) < 2 {
			return errArgs
		}
		remote, inName := config.ParseRemoteAndContainer(args[1])
		if inName == "" {
			return errArgs
		}
		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}
		image := dereferenceAlias(d, inName)
		err = d.DeleteImage(image)
		return err

	case "info":
		if len(args) < 2 {
			return errArgs
		}
		remote, inName := config.ParseRemoteAndContainer(args[1])
		if inName == "" {
			return errArgs
		}
		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}

		image := dereferenceAlias(d, inName)
		info, err := d.GetImageInfo(image)
		if err != nil {
			return err
		}
		fmt.Printf(i18n.G("Fingerprint: %s")+"\n", info.Fingerprint)
		public := i18n.G("no")

		// FIXME: InterfaceToBool is there for backward compatibility
		if shared.InterfaceToBool(info) {
			public = i18n.G("yes")
		}

		fmt.Printf(i18n.G("Size: %.2fMB")+"\n", float64(info.Size)/1024.0/1024.0)
		arch, _ := shared.ArchitectureName(info.Architecture)
		fmt.Printf(i18n.G("Architecture: %s")+"\n", arch)
		fmt.Printf(i18n.G("Public: %s")+"\n", public)
		fmt.Printf(i18n.G("Timestamps:") + "\n")
		const layout = "2006/01/02 15:04 UTC"
		if info.CreationDate != 0 {
			fmt.Printf("    "+i18n.G("Created: %s")+"\n", time.Unix(info.CreationDate, 0).UTC().Format(layout))
		}
		fmt.Printf("    "+i18n.G("Uploaded: %s")+"\n", time.Unix(info.UploadDate, 0).UTC().Format(layout))
		if info.ExpiryDate != 0 {
			fmt.Printf("    "+i18n.G("Expires: %s")+"\n", time.Unix(info.ExpiryDate, 0).UTC().Format(layout))
		} else {
			fmt.Printf("    " + i18n.G("Expires: never") + "\n")
		}
		fmt.Println(i18n.G("Properties:"))
		for key, value := range info.Properties {
			fmt.Printf("    %s: %s\n", key, value)
		}
		fmt.Println(i18n.G("Aliases:"))
		for _, alias := range info.Aliases {
			fmt.Printf("    - %s\n", alias.Name)
		}
		return nil

	case "import":
		if len(args) < 2 {
			return errArgs
		}

		var fingerprint string
		var imageFile string
		var rootfsFile string
		var properties []string
		var remote string

		for _, arg := range args[1:] {
			split := strings.Split(arg, "=")
			if len(split) == 1 || shared.PathExists(arg) {
				if strings.HasSuffix(arg, ":") {
					remote = config.ParseRemote(arg)
				} else {
					if imageFile == "" {
						imageFile = args[1]
					} else {
						rootfsFile = arg
					}
				}
			} else {
				properties = append(properties, arg)
			}
		}

		if remote == "" {
			remote = config.DefaultRemote
		}

		if imageFile == "" {
			return errArgs
		}

		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}

		if strings.HasPrefix(imageFile, "https://") {
			fingerprint, err = d.PostImageURL(imageFile, publicImage, addAliases)
		} else if strings.HasPrefix(imageFile, "http://") {
			return fmt.Errorf(i18n.G("Only https:// is supported for remote image import."))
		} else {
			fingerprint, err = d.PostImage(imageFile, rootfsFile, properties, publicImage, addAliases)
		}

		if err != nil {
			return err
		}
		fmt.Printf(i18n.G("Image imported with fingerprint: %s")+"\n", fingerprint)

		return nil

	case "list":
		if len(args) > 1 {
			remote, _ = config.ParseRemoteAndContainer(args[1])
		} else {
			remote, _ = config.ParseRemoteAndContainer("")
		}

		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}

		images, err := d.ListImages()
		if err != nil {
			return err
		}

		return showImages(images)

	case "edit":
		if len(args) < 2 {
			return errArgs
		}

		remote, inName := config.ParseRemoteAndContainer(args[1])
		if inName == "" {
			return errArgs
		}

		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}

		image := dereferenceAlias(d, inName)
		if image == "" {
			image = inName
		}

		return doImageEdit(d, image)

	case "export":
		if len(args) < 2 {
			return errArgs
		}

		remote, inName := config.ParseRemoteAndContainer(args[1])
		if inName == "" {
			return errArgs
		}

		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}

		image := dereferenceAlias(d, inName)

		target := "."
		if len(args) > 2 {
			target = args[2]
		}
		_, outfile, err := d.ExportImage(image, target)
		if err != nil {
			return err
		}

		if target != "-" {
			fmt.Printf(i18n.G("Output is in %s")+"\n", outfile)
		}
		return nil

	case "show":
		if len(args) < 2 {
			return errArgs
		}
		remote, inName := config.ParseRemoteAndContainer(args[1])
		if inName == "" {
			return errArgs
		}
		d, err := lxd.NewClient(config, remote)
		if err != nil {
			return err
		}

		image := dereferenceAlias(d, inName)
		info, err := d.GetImageInfo(image)
		if err != nil {
			return err
		}

		properties := info.BriefInfo()

		data, err := yaml.Marshal(&properties)
		fmt.Printf("%s", data)
		return err

	default:
		return errArgs
	}
}

func dereferenceAlias(d *lxd.Client, inName string) string {
	result := d.GetAlias(inName)
	if result == "" {
		return inName
	}
	return result
}

func shortestAlias(list shared.ImageAliases) string {
	shortest := ""
	for _, l := range list {
		if shortest == "" {
			shortest = l.Name
			continue
		}
		if len(l.Name) != 0 && len(l.Name) < len(shortest) {
			shortest = l.Name
		}
	}

	return shortest
}

func findDescription(props map[string]string) string {
	for k, v := range props {
		if k == "description" {
			return v
		}
	}
	return ""
}

func showImages(images []shared.ImageInfo) error {
	data := [][]string{}
	for _, image := range images {
		shortest := shortestAlias(image.Aliases)
		if len(image.Aliases) > 1 {
			shortest = fmt.Sprintf(i18n.G("%s (%d more)"), shortest, len(image.Aliases)-1)
		}
		fp := image.Fingerprint[0:12]
		public := i18n.G("no")
		description := findDescription(image.Properties)

		// FIXME: InterfaceToBool is there for backward compatibility
		if shared.InterfaceToBool(image.Public) {
			public = i18n.G("yes")
		}

		const layout = "Jan 2, 2006 at 3:04pm (MST)"
		uploaded := time.Unix(image.UploadDate, 0).Format(layout)
		arch, _ := shared.ArchitectureName(image.Architecture)
		size := fmt.Sprintf("%.2fMB", float64(image.Size)/1024.0/1024.0)
		data = append(data, []string{shortest, fp, public, description, arch, size, uploaded})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(50)
	table.SetHeader([]string{
		i18n.G("ALIAS"),
		i18n.G("FINGERPRINT"),
		i18n.G("PUBLIC"),
		i18n.G("DESCRIPTION"),
		i18n.G("ARCH"),
		i18n.G("SIZE"),
		i18n.G("UPLOAD DATE")})
	sort.Sort(ByName(data))
	table.AppendBulk(data)
	table.Render()

	return nil
}

func showAliases(aliases []shared.ImageAlias) error {
	data := [][]string{}
	for _, alias := range aliases {
		data = append(data, []string{alias.Description, alias.Name[0:12]})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		i18n.G("ALIAS"),
		i18n.G("FINGERPRINT")})

	for _, v := range data {
		table.Append(v)
	}
	table.Render()

	return nil
}

func doImageEdit(client *lxd.Client, image string) error {
	// If stdin isn't a terminal, read text from it
	if !terminal.IsTerminal(int(syscall.Stdin)) {
		contents, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		newdata := shared.BriefImageInfo{}
		err = yaml.Unmarshal(contents, &newdata)
		if err != nil {
			return err
		}
		return client.PutImageInfo(image, newdata)
	}

	// Extract the current value
	config, err := client.GetImageInfo(image)
	if err != nil {
		return err
	}

	brief := config.BriefInfo()
	data, err := yaml.Marshal(&brief)
	if err != nil {
		return err
	}

	// Spawn the editor
	content, err := shared.TextEditor("", []byte(imageEditHelp+"\n\n"+string(data)))
	if err != nil {
		return err
	}

	for {
		// Parse the text received from the editor
		newdata := shared.BriefImageInfo{}
		err = yaml.Unmarshal(content, &newdata)
		if err == nil {
			err = client.PutImageInfo(image, newdata)
		}

		// Respawn the editor
		if err != nil {
			fmt.Fprintf(os.Stderr, i18n.G("Config parsing error: %s")+"\n", err)
			fmt.Println(i18n.G("Press enter to start the editor again"))

			_, err := os.Stdin.Read(make([]byte, 1))
			if err != nil {
				return err
			}

			content, err = shared.TextEditor("", content)
			if err != nil {
				return err
			}
			continue
		}
		break
	}
	return nil
}
