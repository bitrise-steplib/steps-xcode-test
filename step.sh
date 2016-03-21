#!/bin/bash

THIS_SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if [ ! -z "${workdir}" ] ; then
	echo
	echo "=> Switching to working directory: ${workdir}"
	echo "$ cd ${workdir}"
	cd "${workdir}"
fi

export GO15VENDOREXPERIMENT="1"
GOPATH="$THIS_SCRIPT_DIR/go" go run "${THIS_SCRIPT_DIR}/go/src/github.com/bitrise-io/xcode-test/main.go"
exit $?
