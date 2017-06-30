#!/bin/bash
set -xe
# $1 - source dir root
# $2 - $1/debian - just configurable for reuse
# $3 - service definition file
# $4 - service definition target during install
SOURCE_DIR=$1
VERSION=$(${SOURCE_DIR}/../../rpm/version)
RELEASE=$(${SOURCE_DIR}/../../rpm/release)
BUILD_DIR=${SOURCE_DIR}/vpp-monitoring-agent-${VERSION}

# Copy and unpack the archive with vpp-integration distribution
BINARY=${GOPATH}/bin/vpp-monitoring-agent
CONFIG=${GOPATH}/bin/vpp-monitoring-agent-configuration.yaml
SHELL=${GOPATH}/bin/vpp-monitoring-agent.sh
cp ${BINARY} ${SOURCE_DIR}
cp ${CONFIG} ${SOURCE_DIR}
cp ${SHELL} ${SOURCE_DIR}

# Create packaging root
rm -rf ${BUILD_DIR}
mkdir ${BUILD_DIR}

# Copy contents
mv ${SOURCE_DIR}/vpp-monitoring-agent ${BUILD_DIR}/
mv ${SOURCE_DIR}/vpp-monitoring-agent-configuration.yaml ${BUILD_DIR}/
mv ${SOURCE_DIR}/vpp-monitoring-agent.sh ${BUILD_DIR}/
cp -r $2 ${BUILD_DIR}

# OS service definition
cp ${SOURCE_DIR}/$3 ${BUILD_DIR}

# Changelog file
cat <<EOT >> ${BUILD_DIR}/debian/changelog
vpp-monitoring-agent (${VERSION}-${RELEASE}) unstable; urgency=low

  * VPP-MONITORING_AGENT release

 -- Maros Marsalek <maros.mars@gmail.com>  Mon, 27 Feb 2017 09:41:37 +0200
EOT

# Install instructions
cat <<EOT >> ${BUILD_DIR}/debian/install
vpp-monitoring-agent /opt/vpp-monitoring-agent/
vpp-monitoring-agent-configuration.yaml /opt/vpp-monitoring-agent/
vpp-monitoring-agent.sh /opt/vpp-monitoring-agent/
$3 $4
EOT

echo ${BUILD_DIR}