DOMAIN=xlxd
POFILES=$(wildcard po/*.po)
MOFILES=$(patsubst %.po,%.mo,$(POFILES))
LINGUAS=$(basename $(POFILES))
POTFILE=po/$(DOMAIN).pot

# dist is primarily for use when packaging; for development we still manage
# dependencies via `go get` explicitly.
# TODO: use git describe for versioning
VERSION=$(shell grep "var Version" shared/flex.go | sed -r -e 's/.*"([0-9\.]*)"/\1/')
ARCHIVE=xlxd-$(VERSION).tar

.PHONY: default
default:
	# Must run twice due to go get race
	-go get -t -v -d ./...
	-go get -t -v -d ./...
	go install -v ./...
	@echo "XLXD built succesfuly"

.PHONY: client
client:
	# Must run twice due to go get race
	-go get -t -v -d ./...
	-go get -t -v -d ./...
	go install -v ./xlxc
	@echo "XLXD client built succesfuly"

.PHONY: update
update:
	# Must run twice due to go get race
	-go get -t -v -d -u ./...
	go get -t -v -d -u ./...
	@echo "Dependencies updated"

# This only needs to be done when migrate.proto is actually changed; since we
# commit the .pb.go in the tree and it's not expected to change very often,
# it's not a default build step.
.PHONY: protobuf
protobuf:
	protoc --go_out=. ./xlxd/migrate.proto

.PHONY: check
check: default
	go get -v -x github.com/remyoudompheng/go-misc/deadcode
	go get -v -x golang.org/x/tools/cmd/vet
	go test -v ./...
	cd test && ./main.sh

gccgo:
	go build -compiler gccgo ./...
	@echo "XLXD built succesfuly with gccgo"

.PHONY: dist
dist:
	rm -Rf xlxd-$(VERSION) $(ARCHIVE) $(ARCHIVE).gz
	mkdir -p xlxd-$(VERSION)/dist
	-GOPATH=$(shell pwd)/xlxd-$(VERSION)/dist go get -t -v -d ./...
	GOPATH=$(shell pwd)/xlxd-$(VERSION)/dist go get -t -v -d ./...
	rm -rf $(shell pwd)/xlxd-$(VERSION)/dist/src/github.com/krschwab/xlxd
	ln -s ../../../.. ./xlxd-$(VERSION)/dist/src/github.com/krschwab/xlxd
	git archive --prefix=xlxd-$(VERSION)/ --output=$(ARCHIVE) HEAD
	tar -uf $(ARCHIVE) --exclude-vcs xlxd-$(VERSION)/
	gzip -9 $(ARCHIVE)
	rm -Rf dist xlxd-$(VERSION) $(ARCHIVE)

.PHONY: i18n update-po update-pot build-mo static-analysis
i18n: update-pot

po/%.mo: po/%.po
	msgfmt --statistics -o $@ $<

po/%.po: po/$(DOMAIN).pot
	msgmerge -U po/$*.po po/$(DOMAIN).pot

update-po:
	-for lang in $(LINGUAS); do\
	    msgmerge -U $$lang.po po/$(DOMAIN).pot; \
	    rm -f $$lang.po~; \
	done

update-pot:
	go get -v -x github.com/ubuntu-core/snappy/i18n/xgettext-go/
	xgettext-go -o po/$(DOMAIN).pot --add-comments-tag=TRANSLATORS: --sort-output --package-name=$(DOMAIN) --msgid-bugs-address=xlxc-devel@lists.linuxcontainers.org --keyword=i18n.G --keyword-plural=i18n.NG *.go shared/*.go xlxc/*.go xlxd/*.go


build-mo: $(MOFILES)

static-analysis:
	/bin/bash -x -c ". test/static_analysis.sh; static_analysis"

tags: *.go xlxd/*.go shared/*.go xlxc/*.go
	find . | grep \.go | grep -v git | grep -v .swp | grep -v vagrant | xargs gotags > tags
