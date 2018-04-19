#PREFIX is environment variable, but if it is not set, then set default value
ifeq ($(PREFIX),)
    PREFIX := dist
endif

ARCH := $(shell arch)
VERSION := 1.1

all: install

cmd/lgresu_mon/lgresu_mon: cmd/lgresu_mon/lgresu_mon.go
	( go get ./...; cd $(dir $@); go build $(notdir $<) )

doc: doc/LGResuMon.pdf doc/RPISetup.pdf

doc/%.pdf: doc/%.adoc
	asciidoctor-pdf $<


.PHONY: clean
clean:
	-rm cmd/lgresu_mon/lgresu_mon
	-rm -rf dist/*

install: doc/LGResuMon.pdf cmd/lgresu_mon/lgresu_mon
	install -d $(PREFIX)/lgresu-$(VERSION)/bin/
	install -d $(PREFIX)/lgresu-$(VERSION)/doc/
	install -m 644 doc/LGResuMon.pdf $(PREFIX)/lgresu-$(VERSION)/doc
	install -d $(PREFIX)/lgresu-$(VERSION)/script/
	install -m 755 cmd/lgresu_mon/lgresu_mon $(PREFIX)/lgresu-$(VERSION)/bin/lg_resu_mon
	install -d $(PREFIX)/lgresu-$(VERSION)/script/
	install -m 755 script/can_stats.sh $(PREFIX)/lgresu-$(VERSION)/script
	install -m 755 script/keep_alive.sh $(PREFIX)/lgresu-$(VERSION)/script
	install -m 755 script/start_interface.sh $(PREFIX)/lgresu-$(VERSION)/script
	install -m 644 script/lg_resu_dashboard.json $(PREFIX)/lgresu-$(VERSION)/script
	install -m 755 script/start_lg_resu_mon.sh $(PREFIX)/lgresu-$(VERSION)
	tar -c -v -z -f dist/lgresu-$(VERSION)-linux-$(ARCH).tar.gz -C dist  lgresu-$(VERSION)


