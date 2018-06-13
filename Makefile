#PREFIX is environment variable, but if it is not set, then set default value
ifeq ($(PREFIX),)
    PREFIX := dist
endif

ARCH := $(shell go env GOARCH)
VERSION := $(shell git describe)

all: install


# embedding version number:
# 1) https://www.reddit.com/r/golang/comments/4cpi2y/question_where_to_keep_the_version_number_of_a_go/
# 2) https://gist.github.com/TheHippo/7e4d9ec4b7ed4c0d7a39839e6800cc16
cmd/lgresu_mon/lgresu_mon: dep cmd/lgresu_mon/lgresu_mon.go cmd/lgresu_mon/lgresu_actors.go
	( cd $(dir $@); go build -ldflags="-X main.version=${VERSION}" lgresu_mon.go lgresu_actors.go )

.PHONY: dep
dep:
	go get github.com/sirupsen/logrus
	go get github.com/brutella/can
	go get github.com/google/go-cmp/cmp
	go get github.com/gorilla/mux
	go get golang.org/x/tools/cmd/cover
	go get github.com/mattn/goveralls

doc: doc/LGResuMon.pdf doc/RPISetup.pdf

doc/%.pdf: doc/%.adoc
	asciidoctor-pdf $<


.PHONY: clean
clean:
	-rm cmd/lgresu_mon/lgresu_mon
	-rm -rf dist/*

install: cmd/lgresu_mon/lgresu_mon
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



