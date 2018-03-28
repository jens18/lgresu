#PREFIX is environment variable, but if it is not set, then set default value
ifeq ($(PREFIX),)
    PREFIX := dist
endif

all: install

cmd/lgresu_mon/lgresu_mon: cmd/lgresu_mon/lgresu_mon.go
	( cd $(dir $@); go build $(notdir $<) )

doc: doc/LGResuMon.pdf doc/RPISetup.pdf

doc/%.pdf: doc/%.adoc
	asciidoctor-pdf $<


.PHONY: clean
clean:
	-rm doc/*.pdf 
	-rm cmd/lgresu_mon/lgresu_mon
	-rm -rf dist/*

install: doc/LGResuMon.pdf cmd/lgresu_mon/lgresu_mon
	install -d $(PREFIX)/lgresu/bin/
	install -d $(PREFIX)/lgresu/doc/
	install -m 644 doc/LGResuMon.pdf $(PREFIX)/lgresu/doc
	install -d $(PREFIX)/lgresu/script/
	install -m 755 cmd/lgresu_mon/lgresu_mon $(PREFIX)/lgresu/bin/lg_resu_mon
	install -d $(PREFIX)/lgresu/script/
	install -m 755 script/can_stats.sh $(PREFIX)/lgresu/script
	install -m 755 script/keep_alive.sh $(PREFIX)/lgresu/script
	install -m 755 script/start_interface.sh $(PREFIX)/lgresu/script
	install -m 755 script/start_lg_resu_mon.sh $(PREFIX)/lgresu
	tar -c -v -z -f dist/lgresu.tar.gz -C dist  lgresu


