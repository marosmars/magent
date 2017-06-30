#!/bin/bash
set -xe
DIR=$(dirname $0)
DIR=$(readlink -f $DIR)
BINFILE=${GOPATH}/bin/vpp-monitoring-agent
CFGFILE=${GOPATH}/bin/vpp-monitoring-agent-configuration.yaml
SHFILE=${GOPATH}/bin/vpp-monitoring-agent.sh

mkdir -p ${DIR}/SOURCES/
cp $BINFILE ${DIR}/SOURCES/
cp $CFGFILE ${DIR}/SOURCES/
cp $SHFILE ${DIR}/SOURCES/
cp ${DIR}/vpp-monitoring-agent.spec ${DIR}/SOURCES/
cd ${DIR}
rpmbuild -bb --define "_topdir ${DIR}"  ${DIR}/vpp-monitoring-agent.spec
cd -