# XLXD

A FORK OF LXD WITH FEWER RESTRICTIONS

REST API, command line tool and OpenStack integration plugin for LXC.

XLXD is pronounced ex-ell-ex-dee.

## Getting started with XLXD

TBD

## Building from source

TBD

### Building the tools

LXD consists of two binaries, a client called `xlxc` and a server called `xlxd`.
These live in the source tree in the `xlxc/` and `xlxd/` dirs, respectively. 
To get the code, set up your go environment:

    mkdir -p ~/go
    export GOPATH=~/go

And then download it as usual:

    go get github.com/krschwab/xlxd
    cd $GOPATH/src/github.com/krschwab/xlxd
    make

...which will give you two binaries in $GOPATH/bin, `xlxd` the daemon binary,
and `xlxc` a command line client to that daemon.

### Machine Setup

You'll need sub{u,g}ids for root, so that LXD can create the unprivileged
containers:

    echo "root:1000000:65536" | sudo tee -a /etc/subuid /etc/subgid

Now you can run the daemon (the --group sudo bit allows everyone in the sudo
group to talk to LXD; you can create your own group if you want):

    sudo -E $GOPATH/bin/xlxd --group sudo

## First steps

TBD

## Bug reports

Bug reports can be filed at https://github.com/krschwab/lxd/issues/new

## Contributing

Contributions to this project should be sent as pull requests on github.
