#!/bin/bash

set -e

#
# Required parameters
if [ -z "${project_path}" ] ; then
	echo "[!] Missing required input: project_path"
	exit 1
fi

if [ -z "${scheme}" ] ; then
	echo "[!] Missing required input: scheme"
	exit 1
fi

#
# Project-or-Workspace flag
if [[ "${project_path}" == *".xcodeproj" ]]; then
	export CONFIG_xcode_project_action="-project"
elif [[ "${project_path}" == *".xcworkspace" ]]; then
	export CONFIG_xcode_project_action="-workspace"
else
	echo "Failed to get valid project file (invalid project file): ${project_path}"
	exit 1
fi

#
# Device Destination
if [ -z "${simulator_device}" ]; then
  simulator_device='iPad'
fi
if [ -z "${simulator_os_version}" ]; then
  simulator_os_version='latest'
fi
CONFIG_unittest_device_destination="platform=iOS Simulator,name=${simulator_device},OS=${simulator_os_version}"

#
# Print configs
echo " * CONFIG_xcode_project_action: ${CONFIG_xcode_project_action}"
echo " * CONFIG_unittest_device_destination: ${CONFIG_unittest_device_destination}"
echo " * project_path: ${project_path}"
echo " * scheme: ${scheme}"


#
# Main
set -v

xcodebuild ${CONFIG_xcode_project_action} "${project_path}" \
	-scheme "${scheme}" \
	clean test \
	-destination "${CONFIG_unittest_device_destination}" \
	-sdk iphonesimulator \
	-verbose

exit 0
