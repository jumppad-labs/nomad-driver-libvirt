PLUGIN_BINARY=plugin/libvirt
export GO111MODULE=on

.PHONY: clean run build

default: build

clean:
	-rm -rf ${PLUGIN_BINARY}

build: clean
	go build -o ${PLUGIN_BINARY} .

run:
	sudo nomad agent -dev -config=${PWD}/example/config.hcl -plugin-dir=${PWD}/plugin -data-dir=${PWD}/data -bind=0.0.0.0