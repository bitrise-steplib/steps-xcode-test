#!/bin/bash

THIS_SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

set -e

if [ ! -z "${workdir}" ] ; then
	echo
	echo "=> Switching to working directory: ${workdir}"
	echo "$ cd ${workdir}"
	cd "${workdir}"
fi

go run ${THIS_SCRIPTDIR}/step.go
