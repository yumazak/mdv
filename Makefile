.PHONY: build install vendor clean

build:
	go build -o mdv .

install: build
	ln -sf $(CURDIR)/mdv $(HOME)/.local/bin/mdv

vendor:
	./fetch-vendor.sh

clean:
	rm -f mdv
